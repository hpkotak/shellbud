// Package prompt handles LLM prompt construction and response parsing.
// The parser is the reliability layer — it defensively handles LLM output
// quirks (code fences, backticks, explanatory text) regardless of how well
// the system prompt constrains the model.
package prompt

import (
	"fmt"
	"regexp"
	"strings"
)

// ChatSystemPrompt returns the system prompt for interactive assistant mode.
// envContext is the formatted environment snapshot from shellenv.Snapshot.Format().
func ChatSystemPrompt(envContext string) string {
	return fmt.Sprintf(`You are ShellBud, a shell assistant. You help users interact with their shell using natural language.

Current environment:
%s
Guidelines:
- When suggesting a command, put it in a fenced code block (triple backticks).
- You can explain, suggest commands, answer questions, or do all three.
- Use standard tools available on the user's OS.
- Prefer common, well-known commands over obscure alternatives.
- Be concise. Don't over-explain unless asked.
- If a task requires multiple steps, suggest them one at a time.`, envContext)
}

// ParsedResponse represents a structured LLM chat response.
type ParsedResponse struct {
	Text     string   // full response text for display
	Commands []string // commands extracted from code blocks
}

// codeBlockRe matches fenced code blocks: ```lang\n...\n``` or ```\n...\n```
var codeBlockRe = regexp.MustCompile("(?s)```[a-zA-Z]*\\n(.*?)```")

// ParseChatResponse parses an LLM chat response, extracting any commands
// from fenced code blocks while preserving the full text.
func ParseChatResponse(raw string) ParsedResponse {
	text := strings.TrimSpace(raw)
	if text == "" {
		return ParsedResponse{}
	}

	var commands []string
	matches := codeBlockRe.FindAllStringSubmatch(text, -1)
	for _, m := range matches {
		cmd := strings.TrimSpace(m[1])
		if cmd != "" {
			commands = append(commands, cmd)
		}
	}

	return ParsedResponse{
		Text:     text,
		Commands: commands,
	}
}
