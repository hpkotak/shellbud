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

// Chat sends the conversation to Ollama and returns the assistant's response.
// Converts provider.Message to api.Message internally so callers stay decoupled
// from the Ollama client library.
func (o *OllamaProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	apiMessages := make([]api.Message, len(messages))
	for i, m := range messages {
		apiMessages[i] = api.Message{Role: m.Role, Content: m.Content}
	}

	stream := false
	req := &api.ChatRequest{
		Model:    o.model,
		Messages: apiMessages,
		Stream:   &stream,
		Format:   json.RawMessage(`"json"`),
	}

	var result string
	err := o.client.Chat(ctx, req, func(resp api.ChatResponse) error {
		result = resp.Message.Content
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("ollama chat: %w", err)
	}

	if strings.TrimSpace(result) == "" {
		return "", fmt.Errorf("empty response from model")
	}
	return result, nil
}
