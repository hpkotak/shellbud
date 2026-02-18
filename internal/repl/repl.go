// Package repl implements the interactive chat loop for ShellBud.
// It manages conversation history, environment context refresh, and
// the run/explain/skip command interaction flow.
//
// Environment context is refreshed every turn (not cached) because the user's
// shell state changes between prompts (cd, git operations, file creation).
// History is capped rather than summarized — token limits are the LLM's problem
// via context window, not ours.
package repl

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/hpkotak/shellbud/internal/executor"
	"github.com/hpkotak/shellbud/internal/prompt"
	"github.com/hpkotak/shellbud/internal/provider"
	"github.com/hpkotak/shellbud/internal/safety"
	"github.com/hpkotak/shellbud/internal/shellenv"
)

const (
	chatTimeout    = 120 * time.Second
	maxHistoryMsgs = 50
)

// Package-level function variables for testability.
var (
	runCapture = executor.RunCapture
	gatherEnv  = shellenv.Gather
)

// Run starts the interactive REPL loop.
func Run(p provider.Provider, in io.Reader, out io.Writer) error {
	_, _ = fmt.Fprintln(out, "ShellBud Chat (type 'exit' to quit)")
	_, _ = fmt.Fprintln(out)

	scanner := bufio.NewScanner(in)
	var history []provider.Message

	for {
		_, _ = fmt.Fprint(out, "sb> ")

		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				_, _ = fmt.Fprintf(out, "\nInput error: %v\n", err)
				return err
			}
			_, _ = fmt.Fprintln(out)
			break // EOF (Ctrl+D)
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == "exit" || input == "quit" {
			_, _ = fmt.Fprintln(out, "Bye!")
			return nil
		}

		// Refresh environment context each turn.
		envSnap := gatherEnv()
		sysMsg := provider.Message{
			Role:    "system",
			Content: prompt.ChatSystemPrompt(envSnap.Format()),
		}

		// Add user message to history.
		history = append(history, provider.Message{Role: "user", Content: input})

		// Trim history if too long (keep most recent messages).
		if len(history) > maxHistoryMsgs {
			history = history[len(history)-maxHistoryMsgs:]
		}

		// Build full message list: system + history.
		messages := make([]provider.Message, 0, 1+len(history))
		messages = append(messages, sysMsg)
		messages = append(messages, history...)

		result, err := sendMessage(p, messages)
		if err != nil {
			_, _ = fmt.Fprintf(out, "Error: %v\n\n", err)
			continue
		}

		if result.Warning != "" {
			_, _ = fmt.Fprintf(out, "\n  Note: %s\n", result.Warning)
		}

		// Add assistant response to history.
		history = append(history, provider.Message{Role: "assistant", Content: result.Text})

		parsed := prompt.ParseChatResponse(result.Text)

		// Display the full response.
		_, _ = fmt.Fprintf(out, "\n%s\n", parsed.Text)
		if !parsed.Structured {
			_, _ = fmt.Fprintln(out, "  Note: model response was not valid structured output; no commands were run.")
		}

		// Handle any extracted commands.
		for _, command := range parsed.Commands {
			handleCommand(command, &history, sysMsg, p, scanner, out)
		}

		_, _ = fmt.Fprintln(out)
	}

	return nil
}

func sendMessage(p provider.Provider, messages []provider.Message) (provider.ChatResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), chatTimeout)
	defer cancel()

	return p.Chat(ctx, provider.ChatRequest{
		Messages:   messages,
		ExpectJSON: true,
	})
}

func handleCommand(command string, history *[]provider.Message, sysMsg provider.Message, p provider.Provider, scanner *bufio.Scanner, out io.Writer) {
	level := safety.Classify(command)

	_, _ = fmt.Fprintf(out, "\n  > %s\n", command)

	if level == safety.Destructive {
		_, _ = fmt.Fprintln(out, "  Warning: destructive command")
	}

	_, _ = fmt.Fprint(out, "  [r]un / [e]xplain / [s]kip: ")

	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			_, _ = fmt.Fprintf(out, "  Input error: %v\n", err)
		}
		return
	}
	choice := strings.TrimSpace(strings.ToLower(scanner.Text()))

	switch choice {
	case "r", "run":
		if level == safety.Destructive {
			_, _ = fmt.Fprint(out, "  Are you sure? [y/N]: ")
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					_, _ = fmt.Fprintf(out, "  Input error: %v\n", err)
				}
				return
			}
			confirm := strings.TrimSpace(strings.ToLower(scanner.Text()))
			if confirm != "y" && confirm != "yes" {
				_, _ = fmt.Fprintln(out, "  Skipped.")
				return
			}
		}

		_, _ = fmt.Fprintln(out)
		output, exitCode, err := runCapture(command)
		if err != nil {
			_, _ = fmt.Fprintf(out, "  Execution error: %v\n", err)
			return
		}

		// Add command output to conversation context.
		contextMsg := fmt.Sprintf("I ran `%s` — exit code %d.", command, exitCode)
		if output != "" {
			contextMsg += fmt.Sprintf("\nOutput:\n```\n%s\n```", output)
		}
		*history = append(*history, provider.Message{
			Role:    "user",
			Content: contextMsg,
		})

	case "e", "explain":
		explainMsg := provider.Message{
			Role:    "user",
			Content: fmt.Sprintf("Explain what this command does step by step: `%s`", command),
		}
		*history = append(*history, explainMsg)

		// Build messages for immediate LLM call.
		messages := make([]provider.Message, 0, 1+len(*history))
		messages = append(messages, sysMsg)
		messages = append(messages, *history...)

		result, err := sendMessage(p, messages)
		if err != nil {
			_, _ = fmt.Fprintf(out, "  Explain error: %v\n", err)
			return
		}

		*history = append(*history, provider.Message{Role: "assistant", Content: result.Text})
		explanation := prompt.ParseChatResponse(result.Text).Text
		_, _ = fmt.Fprintf(out, "\n%s\n", explanation)

	case "s", "skip", "":
		_, _ = fmt.Fprintln(out, "  Skipped.")
	default:
		_, _ = fmt.Fprintln(out, "  Skipped.")
	}
}
