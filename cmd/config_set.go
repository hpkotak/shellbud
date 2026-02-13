package cmd

import (
	"fmt"

	"github.com/hpkotak/shellbud/internal/config"
	"github.com/spf13/cobra"
)

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Update a configuration value",
	Long: `Update a configuration value. Supported keys:
  provider    LLM provider (ollama)
  model       Model name (e.g., llama3.2:latest)
  ollama.host Ollama server URL`,
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
		cfg = config.Default()
	}

	switch key {
	case "provider":
		cfg.Provider = value
	case "model":
		cfg.Model = value
	case "ollama.host":
		cfg.Ollama.Host = value
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}

	if err := config.Save(cfg); err != nil {
		return err
	}

	fmt.Printf("Set %s = %s\n", key, value)
	return nil
}
