package provider

import (
	"fmt"
	"strings"
)

// BuildConfig contains provider-specific runtime settings used by the factory.
type BuildConfig struct {
	Name         string
	Model        string
	OllamaHost   string
	OpenAIHost   string
	OpenAIAPIKey string
	AFMCommand   string
}

// NewFromConfig builds the configured provider implementation.
func NewFromConfig(cfg BuildConfig) (Provider, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Name)) {
	case "ollama":
		return NewOllama(cfg.OllamaHost, cfg.Model)
	case "openai":
		return NewOpenAI(cfg.OpenAIHost, cfg.Model, cfg.OpenAIAPIKey)
	case "afm":
		return NewAFM(cfg.Model, cfg.AFMCommand)
	default:
		return nil, fmt.Errorf("unsupported provider %q", cfg.Name)
	}
}
