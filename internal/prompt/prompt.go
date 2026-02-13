// Package prompt handles LLM prompt construction and response parsing.
// The parser is the reliability layer — it defensively handles LLM output
// quirks (code fences, backticks, explanatory text) regardless of how well
// the system prompt constrains the model.
package prompt

import (
	"fmt"
	"strings"
)

// SystemPrompt returns the system prompt that instructs the LLM to translate
// natural language to shell commands.
func SystemPrompt() string {
	return `You are a shell command translator. Given a natural language description, return ONLY the shell command that accomplishes the task. No explanation, no markdown, no code fences, no commentary. Just the raw command.

Rules:
- Output exactly one line: the shell command.
- Use standard POSIX/GNU tools available on the target OS.
- Prefer common, well-known commands over obscure alternatives.
- If the request is ambiguous, pick the most common interpretation.
- Never output anything besides the command itself.`
}

// BuildUserPrompt constructs the user prompt with OS and shell context.
func BuildUserPrompt(query, osName, shell string) string {
	return fmt.Sprintf("OS: %s, Shell: %s\nTranslate to shell command: %s", osName, shell, query)
}

// ParseResponse cleans up the LLM response to extract just the command.
// Handles common LLM quirks: code fences, backticks, preamble, "$ " prefix.
func ParseResponse(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}

	// Strip markdown code fences (```sh ... ``` or ```bash ... ``` or ``` ... ```)
	if strings.HasPrefix(s, "```") {
		lines := strings.Split(s, "\n")
		// Remove first line (```sh, ```bash, ```)
		if len(lines) > 1 {
			lines = lines[1:]
		}
		// Remove last line if it's closing fence
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "```" {
			lines = lines[:len(lines)-1]
		}
		s = strings.TrimSpace(strings.Join(lines, "\n"))
	}

	// Strip inline backticks
	s = strings.Trim(s, "`")
	s = strings.TrimSpace(s)

	// Take only the first non-empty line (ignore explanations after the command)
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			s = line
			break
		}
	}

	// Strip leading "$ " prompt marker
	s = strings.TrimPrefix(s, "$ ")

	return s
}
