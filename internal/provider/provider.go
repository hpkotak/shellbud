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

// ChatRequest represents a normalized LLM request.
type ChatRequest struct {
	Messages   []Message
	Model      string
	ExpectJSON bool
}

// Usage represents token usage metadata when available.
type Usage struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
}

// ChatResponse is a normalized provider response.
type ChatResponse struct {
	// Text is the assistant content used for user display and prompt parsing.
	Text string
	// Raw is provider-native payload content for debugging/inspection.
	Raw string
	// Structured indicates valid JSON content when ExpectJSON is true.
	Structured bool
	// FinishReason is provider stop reason, when available.
	FinishReason string
	// Usage is token usage metadata, when available.
	Usage Usage
	// Warning is an optional provider-level notice (e.g., context was trimmed).
	// Callers should display this separately from Text to avoid corrupting
	// structured response parsing.
	Warning string
}

// Capabilities describes optional provider features.
type Capabilities struct {
	JSONMode     bool
	Usage        bool
	FinishReason bool
}

// Provider sends conversations to an LLM backend.
type Provider interface {
	// Chat sends the request and returns a normalized provider response.
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)

	// Name returns the provider name (e.g., "ollama").
	Name() string

	// Capabilities returns feature support for the provider backend.
	Capabilities() Capabilities

	// Available checks if this provider is ready to use.
	Available(ctx context.Context) error
}
