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
	"github.com/hpkotak/shellbud/internal/platform"
	"github.com/hpkotak/shellbud/internal/provider"
	"github.com/hpkotak/shellbud/internal/safety"
	"github.com/spf13/cobra"
)

var modelFlag string

// Package-level function variables for testability.
// Tests override these to avoid real provider/executor calls.
var (
	newProvider = func(host, model string) (provider.Provider, error) {
		return provider.NewOllama(host, model)
	}
	runCommand = executor.Run
	ioIn       io.Reader = os.Stdin
	ioOut      io.Writer = os.Stdout
)

var rootCmd = &cobra.Command{
	Use:   "sb [natural language query]",
	Short: "Translate natural language to shell commands",
	Long: `ShellBud (sb) translates natural language descriptions into shell commands.

Examples:
  sb compress this folder as tar.gz
  sb find all files larger than 100MB
  sb show disk usage sorted by size`,
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

	result, err := p.Translate(ctx, query, platform.OS(), platform.Shell())
	if err != nil {
		return fmt.Errorf("translation failed: %w", err)
	}

	_, _ = fmt.Fprintf(ioOut, "\n  %s\n\n", result)

	level := safety.Classify(result)

	var confirmed bool
	if level == safety.Destructive {
		_, _ = fmt.Fprintln(ioOut, "  Warning: this is a destructive command.")
		confirmed = executor.Confirm("  Are you sure?", false, ioIn, ioOut)
	} else {
		confirmed = executor.Confirm("  Run this?", true, ioIn, ioOut)
	}

	if !confirmed {
		_, _ = fmt.Fprintln(ioOut, "  Cancelled.")
		return nil
	}

	_, _ = fmt.Fprintln(ioOut)
	return runCommand(result)
}
