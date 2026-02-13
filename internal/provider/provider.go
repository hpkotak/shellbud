// Package provider defines the LLM backend interface and implementations.
// New backends (Apple FM, Claude API) implement the Provider interface.
package provider

import "context"

// Provider translates natural language to a shell command.
type Provider interface {
	// Translate sends the user's natural language query and returns the shell command.
	// osName and shell are passed by the caller so providers stay decoupled from platform detection.
	Translate(ctx context.Context, query, osName, shell string) (string, error)

	// Name returns the provider name (e.g., "ollama").
	Name() string

	// Available checks if this provider is ready to use.
	Available(ctx context.Context) error
}
