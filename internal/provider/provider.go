// Package provider defines the LLM backend interface and implementations.
// New backends (Apple FM, Claude API) implement the Provider interface.
package provider

import "context"

// Message represents a single message in a conversation.
// Decoupled from any specific LLM API (Ollama, OpenAI, etc.) so callers
// don't import backend-specific types.
type Message struct {
	Role    string // "system", "user", "assistant"
	Content string
}

// Provider sends conversations to an LLM backend.
type Provider interface {
	// Chat sends the message history and returns the assistant's response.
	Chat(ctx context.Context, messages []Message) (string, error)

	// Name returns the provider name (e.g., "ollama").
	Name() string

	// Available checks if this provider is ready to use.
	Available(ctx context.Context) error
}
