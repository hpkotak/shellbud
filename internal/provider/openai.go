package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const openAIErrorBodyLimit = 512

// OpenAIProvider implements Provider using the OpenAI Chat Completions API.
type OpenAIProvider struct {
	client *http.Client
	host   string
	model  string
	apiKey string
}

// NewOpenAI creates an OpenAIProvider connected to the given host and model.
func NewOpenAI(host, model, apiKey string) (*OpenAIProvider, error) {
	base := strings.TrimSpace(host)
	if base == "" {
		return nil, fmt.Errorf("openai host cannot be empty")
	}
	if _, err := url.ParseRequestURI(base); err != nil {
		return nil, fmt.Errorf("parsing openai host URL: %w", err)
	}
	if strings.TrimSpace(model) == "" {
		return nil, fmt.Errorf("model cannot be empty")
	}
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("openai api key is required (set OPENAI_API_KEY)")
	}

	return &OpenAIProvider{
		client: &http.Client{Timeout: 30 * time.Second},
		host:   strings.TrimRight(base, "/"),
		model:  model,
		apiKey: apiKey,
	}, nil
}

func (o *OpenAIProvider) Name() string { return "openai" }

func (o *OpenAIProvider) Capabilities() Capabilities {
	return Capabilities{
		JSONMode:     true,
		Usage:        true,
		FinishReason: true,
	}
}

// Available checks if OpenAI is reachable and the configured model exists.
func (o *OpenAIProvider) Available(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.host+"/models", nil)
	if err != nil {
		return fmt.Errorf("building openai availability request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := o.client.Do(req)
	if err != nil {
		return fmt.Errorf("checking openai availability: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("openai availability check failed: %s", readErrorBody(resp.Body))
	}

	var decoded struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return fmt.Errorf("decoding openai models response: %w", err)
	}

	for _, m := range decoded.Data {
		if m.ID == o.model {
			return nil
		}
	}
	return fmt.Errorf("model %q not found in OpenAI models list", o.model)
}

// Chat sends the conversation to OpenAI and returns the assistant response.
// Request JSON-mode is enforced with response_format.type=json_object.
func (o *OpenAIProvider) Chat(ctx context.Context, chatReq ChatRequest) (ChatResponse, error) {
	type openAIMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	type openAIChatRequest struct {
		Model          string          `json:"model"`
		Messages       []openAIMessage `json:"messages"`
		ResponseFormat *struct {
			Type string `json:"type"`
		} `json:"response_format,omitempty"`
	}

	apiMessages := make([]openAIMessage, len(chatReq.Messages))
	for i, m := range chatReq.Messages {
		apiMessages[i] = openAIMessage(m)
	}

	model := resolveModel(chatReq.Model, o.model)

	var reqBody openAIChatRequest
	reqBody.Model = model
	reqBody.Messages = apiMessages
	if chatReq.ExpectJSON {
		reqBody.ResponseFormat = &struct {
			Type string `json:"type"`
		}{
			Type: "json_object",
		}
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("encoding openai chat request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		o.host+"/chat/completions",
		bytes.NewReader(body),
	)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("building openai chat request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+o.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("openai chat: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusBadRequest {
		return ChatResponse{}, fmt.Errorf("openai chat failed: %s", readErrorBody(resp.Body))
	}

	var decoded struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return ChatResponse{}, fmt.Errorf("decoding openai chat response: %w", err)
	}
	if len(decoded.Choices) == 0 {
		return ChatResponse{}, fmt.Errorf("empty response from model")
	}

	result := strings.TrimSpace(decoded.Choices[0].Message.Content)
	if result == "" {
		return ChatResponse{}, fmt.Errorf("empty response from model")
	}

	usage := Usage{
		InputTokens:  decoded.Usage.PromptTokens,
		OutputTokens: decoded.Usage.CompletionTokens,
		TotalTokens:  decoded.Usage.TotalTokens,
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	}

	return ChatResponse{
		Text:         result,
		Raw:          result,
		Structured:   isStructuredJSON(chatReq.ExpectJSON, result),
		FinishReason: decoded.Choices[0].FinishReason,
		Usage:        usage,
	}, nil
}

func readErrorBody(r io.Reader) string {
	body, _ := io.ReadAll(io.LimitReader(r, openAIErrorBodyLimit))
	text := strings.TrimSpace(string(body))
	if text == "" {
		return "unknown error"
	}
	return text
}
