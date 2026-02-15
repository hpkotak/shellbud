package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/hpkotak/shellbud/internal/config"
	"github.com/hpkotak/shellbud/internal/executor"
	"github.com/hpkotak/shellbud/internal/prompt"
	"github.com/hpkotak/shellbud/internal/provider"
	"github.com/hpkotak/shellbud/internal/safety"
	"github.com/hpkotak/shellbud/internal/shellenv"
	"github.com/spf13/cobra"
)

var modelFlag string

// Package-level function variables for testability.
// Tests override these to avoid real provider/executor calls.
var (
	newProvider = func(host, model string) (provider.Provider, error) {
		return provider.NewOllama(host, model)
	}
	runCommand           = executor.Run
	ioIn       io.Reader = os.Stdin
	ioOut      io.Writer = os.Stdout
)

var rootCmd = &cobra.Command{
	Use:   "sb [natural language query]",
	Short: "Your shell buddy — ask questions, get commands",
	Long: `ShellBud (sb) is a context-aware shell assistant.
Ask questions in natural language and get commands tailored to your environment.

Examples:
  sb what git branch am I on
  sb find all files larger than 100MB
  sb compress this folder as tar.gz

For interactive sessions: sb chat`,
	Args:              cobra.ArbitraryArgs,
	RunE:              runTranslate,
	DisableAutoGenTag: true,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&modelFlag, "model", "", "override model for this query")
}

func Execute() error {
	return rootCmd.Execute()
}

func runTranslate(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Help()
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("no config found. Run 'sb setup' to get started")
	}

	model := cfg.Model
	if modelFlag != "" {
		model = modelFlag
	}

	p, err := newProvider(cfg.Ollama.Host, model)
	if err != nil {
		return fmt.Errorf("creating provider: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	query := strings.Join(args, " ")
	envSnap := shellenv.Gather()

	messages := []provider.Message{
		{Role: "system", Content: prompt.ChatSystemPrompt(envSnap.Format())},
		{Role: "user", Content: query},
	}

	result, err := p.Chat(ctx, messages)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	parsed := prompt.ParseChatResponse(result)

	// Display the full response text.
	_, _ = fmt.Fprintf(ioOut, "\n%s\n", parsed.Text)
	if !parsed.Structured {
		_, _ = fmt.Fprintln(ioOut, "  Note: model response was not valid structured output; no commands were run.")
	}

	// If commands were extracted, offer to run each one.
	for _, command := range parsed.Commands {
		_, _ = fmt.Fprintf(ioOut, "\n  > %s\n\n", command)

		level := safety.Classify(command)

		var confirmed bool
		if level == safety.Destructive {
			_, _ = fmt.Fprintln(ioOut, "  Warning: this is a destructive command.")
			confirmed = executor.Confirm("  Are you sure?", false, ioIn, ioOut)
		} else {
			confirmed = executor.Confirm("  Run this?", true, ioIn, ioOut)
		}

		if !confirmed {
			_, _ = fmt.Fprintln(ioOut, "  Skipped.")
			continue
		}

		_, _ = fmt.Fprintln(ioOut)
		if err := runCommand(command); err != nil {
			return err
		}
	}

	return nil
}
