package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/hpkotak/shellbud/internal/prompt"
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

// Translate sends the query to Ollama and returns the shell command.
// Uses Generate (not Chat) because each translation is stateless — no
// conversation history needed. The callback fires once with Stream=false.
func (o *OllamaProvider) Translate(ctx context.Context, query, osName, shell string) (string, error) {
	stream := false
	userPrompt := prompt.BuildUserPrompt(query, osName, shell)

	req := &api.GenerateRequest{
		Model:  o.model,
		System: prompt.SystemPrompt(),
		Prompt: userPrompt,
		Stream: &stream,
	}

	var result string
	err := o.client.Generate(ctx, req, func(resp api.GenerateResponse) error {
		result = resp.Response
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("ollama generate: %w", err)
	}

	parsed := prompt.ParseResponse(result)
	if parsed == "" {
		return "", fmt.Errorf("empty response from model")
	}
	return parsed, nil
}
