package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/hpkotak/shellbud/internal/config"
	"github.com/hpkotak/shellbud/internal/executor"
	"github.com/hpkotak/shellbud/internal/provider"
	"github.com/hpkotak/shellbud/internal/safety"
	"github.com/spf13/cobra"
)

var modelFlag string

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
		fmt.Fprintln(os.Stderr, "No config found. Run 'sb setup' to get started.")
		os.Exit(1)
	}

	model := cfg.Model
	if modelFlag != "" {
		model = modelFlag
	}

	p, err := provider.NewOllama(cfg.Ollama.Host, model)
	if err != nil {
		return fmt.Errorf("creating provider: %w", err)
	}

	ctx := context.Background()
	query := strings.Join(args, " ")

	result, err := p.Translate(ctx, query)
	if err != nil {
		return fmt.Errorf("translation failed: %w", err)
	}

	fmt.Printf("\n  %s\n\n", result)

	level := safety.Classify(result)

	var confirmed bool
	if level == safety.Destructive {
		fmt.Println("  Warning: this is a destructive command.")
		confirmed = executor.Confirm("  Are you sure?", false, os.Stdin, os.Stdout)
	} else {
		confirmed = executor.Confirm("  Run this?", true, os.Stdin, os.Stdout)
	}

	if !confirmed {
		fmt.Println("  Cancelled.")
		return nil
	}

	fmt.Println()
	return executor.Run(result)
}
