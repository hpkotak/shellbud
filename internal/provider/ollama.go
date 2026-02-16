package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ollama/ollama/api"
)

// OllamaProvider implements Provider using a local Ollama instance.
type OllamaProvider struct {
	client *api.Client
	model  string
}

// NewOllama creates an OllamaProvider connected to the given host and model.
func NewOllama(host, model string) (*OllamaProvider, error) {
	base, err := url.Parse(host)
	if err != nil {
		return nil, fmt.Errorf("parsing ollama host URL: %w", err)
	}
	httpClient := &http.Client{Timeout: 30 * time.Second}
	client := api.NewClient(base, httpClient)
	return &OllamaProvider{client: client, model: model}, nil
}

func (o *OllamaProvider) Name() string { return "ollama" }

func (o *OllamaProvider) Capabilities() Capabilities {
	return Capabilities{
		JSONMode:     true,
		Usage:        true,
		FinishReason: true,
	}
}

// Available checks if Ollama is reachable and the configured model exists.
func (o *OllamaProvider) Available(ctx context.Context) error {
	models, err := o.client.List(ctx)
	if err != nil {
		return fmt.Errorf("cannot reach Ollama at configured host: %w", err)
	}

	for _, m := range models.Models {
		if m.Name == o.model {
			return nil
		}
	}
	return fmt.Errorf("model %q not found in Ollama", o.model)
}

// Chat sends the conversation to Ollama and returns the assistant response.
// Converts provider.Message to api.Message internally so callers stay decoupled
// from the Ollama client library.
func (o *OllamaProvider) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	apiMessages := make([]api.Message, len(req.Messages))
	for i, m := range req.Messages {
		apiMessages[i] = api.Message{Role: m.Role, Content: m.Content}
	}

	model := resolveModel(req.Model, o.model)

	stream := false
	ollamaReq := &api.ChatRequest{
		Model:    model,
		Messages: apiMessages,
		Stream:   &stream,
	}
	if req.ExpectJSON {
		ollamaReq.Format = json.RawMessage(`"json"`)
	}

	var finalResp api.ChatResponse
	err := o.client.Chat(ctx, ollamaReq, func(resp api.ChatResponse) error {
		finalResp = resp
		return nil
	})
	if err != nil {
		return ChatResponse{}, fmt.Errorf("ollama chat: %w", err)
	}

	result := finalResp.Message.Content
	if strings.TrimSpace(result) == "" {
		return ChatResponse{}, fmt.Errorf("empty response from model")
	}

	usage := Usage{
		InputTokens:  finalResp.PromptEvalCount,
		OutputTokens: finalResp.EvalCount,
	}
	usage.TotalTokens = usage.InputTokens + usage.OutputTokens

	return ChatResponse{
		Text:         result,
		Raw:          result,
		Structured:   isStructuredJSON(req.ExpectJSON, result),
		FinishReason: finalResp.DoneReason,
		Usage:        usage,
	}, nil
}
