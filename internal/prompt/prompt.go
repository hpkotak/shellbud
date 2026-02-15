// Package prompt handles LLM prompt construction and response parsing.
// The parser enforces a structured response contract so command execution
// remains deterministic and fail-closed even when model output drifts.
package prompt

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ChatSystemPrompt returns the system prompt for interactive assistant mode.
// envContext is the formatted environment snapshot from shellenv.Snapshot.Format().
func ChatSystemPrompt(envContext string) string {
	return fmt.Sprintf(`You are ShellBud, a shell assistant. You help users interact with their shell using natural language.

Current environment:
%s
Guidelines:
- Respond with ONLY valid JSON. Do not include markdown or code fences.
- Use this exact schema: {"text":"...","commands":["..."]}.
- The "text" field is concise, user-facing guidance.
- The "commands" field contains zero or more executable shell commands.
- Use an empty array when no command should be run.
- Use standard tools available on the user's OS.
- Prefer common, well-known commands over obscure alternatives.
- Be concise. Don't over-explain unless asked.
- If a task requires multiple steps, suggest them one at a time.`, envContext)
}

// ParsedResponse represents a structured LLM chat response.
type ParsedResponse struct {
	Text     string   // full response text for display
	Commands []string // executable commands from structured JSON only
	// Structured is true only when the response was valid structured JSON.
	Structured bool
}

type structuredChatResponse struct {
	Text     string   `json:"text"`
	Commands []string `json:"commands"`
}

// ParseChatResponse parses an LLM chat response. Commands are accepted only
// from valid structured JSON to keep execution fail-closed.
func ParseChatResponse(raw string) ParsedResponse {
	text := strings.TrimSpace(raw)
	if text == "" {
		return ParsedResponse{}
	}

	parsed, err := parseStructuredChatResponse(text)
	if err != nil {
		// Fail closed: preserve text for display, but offer no runnable commands.
		return ParsedResponse{Text: text, Structured: false}
	}

	return parsed
}

func parseStructuredChatResponse(text string) (ParsedResponse, error) {
	var decoded structuredChatResponse
	if err := json.Unmarshal([]byte(text), &decoded); err != nil {
		return ParsedResponse{}, err
	}

	commands := normalizeCommands(decoded.Commands)
	displayText := strings.TrimSpace(decoded.Text)
	if displayText == "" {
		displayText = text
	}

	return ParsedResponse{
		Text:       displayText,
		Commands:   commands,
		Structured: true,
	}, nil
}

func normalizeCommands(commands []string) []string {
	var out []string
	for _, cmd := range commands {
		trimmed := strings.TrimSpace(cmd)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}
