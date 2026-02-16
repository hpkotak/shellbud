package cmd

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/hpkotak/shellbud/internal/config"
	"github.com/spf13/cobra"
)

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Update a configuration value",
	Long: `Update a configuration value. Supported keys:
  provider    LLM provider (ollama/openai/afm)
  model       Model name (e.g., llama3.2:latest)
  ollama.host Ollama server URL
  openai.host OpenAI-compatible API base URL
  afm.command AFM bridge executable path`,
	Args: cobra.ExactArgs(2),
	RunE: runConfigSet,
}

func init() {
	configCmd.AddCommand(configSetCmd)
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key, value := args[0], args[1]

	cfg, err := config.Load()
	if err != nil {
		if !errors.Is(err, config.ErrNotFound) {
			return fmt.Errorf("loading config: %w", err)
		}
		cfg = config.Default()
	}

	switch key {
	case "provider":
		cfg.Provider = value
		applyProviderDefaults(cfg)
	case "model":
		value = strings.TrimSpace(value)
		if value == "" {
			return fmt.Errorf("model cannot be empty")
		}
		cfg.Model = value
	case "ollama.host":
		if _, err := url.ParseRequestURI(value); err != nil {
			return fmt.Errorf("invalid URL %q: %w", value, err)
		}
		cfg.Ollama.Host = value
	case "openai.host":
		if _, err := url.ParseRequestURI(value); err != nil {
			return fmt.Errorf("invalid URL %q: %w", value, err)
		}
		cfg.OpenAI.Host = value
	case "afm.command":
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("afm command cannot be empty")
		}
		cfg.AFM.Command = value
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}

	if err := cfg.Validate(); err != nil {
		return err
	}

	if err := config.Save(cfg); err != nil {
		return err
	}

	fmt.Printf("Set %s = %s\n", key, value)
	return nil
}

func applyProviderDefaults(cfg *config.Config) {
	defaults := config.Default()
	switch cfg.Provider {
	case "ollama":
		if strings.TrimSpace(cfg.Ollama.Host) == "" {
			cfg.Ollama.Host = defaults.Ollama.Host
		}
	case "openai":
		if strings.TrimSpace(cfg.OpenAI.Host) == "" {
			cfg.OpenAI.Host = defaults.OpenAI.Host
		}
	case "afm":
		if strings.TrimSpace(cfg.AFM.Command) == "" {
			cfg.AFM.Command = defaults.AFM.Command
		}
	}
}
