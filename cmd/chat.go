package cmd

import (
	"errors"
	"fmt"

	"github.com/hpkotak/shellbud/internal/config"
	"github.com/hpkotak/shellbud/internal/repl"
	"github.com/spf13/cobra"
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive shell assistant session",
	Long: `Start an interactive chat session with ShellBud.
Ask questions, get commands, run them, and continue the conversation.

Type 'exit' or 'quit' to end the session. Ctrl+D also works.`,
	RunE: runChat,
}

func init() {
	rootCmd.AddCommand(chatCmd)
}

func runChat(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		if errors.Is(err, config.ErrNotFound) {
			return fmt.Errorf("no config found. Run 'sb setup' to get started")
		}
		return fmt.Errorf("loading config: %w", err)
	}

	model := cfg.Model
	if modelFlag != "" {
		model = modelFlag
	}

	p, err := newProvider(cfg, model)
	if err != nil {
		return fmt.Errorf("creating provider: %w", err)
	}

	return repl.Run(p, ioIn, ioOut)
}
