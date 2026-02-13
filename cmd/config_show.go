package cmd

import (
	"fmt"

	"github.com/hpkotak/shellbud/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	RunE:  runConfigShow,
}

func init() {
	configCmd.AddCommand(configShowCmd)
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("no config found. Run 'sb setup' first")
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}

	fmt.Printf("Config file: %s\n\n", config.Path())
	fmt.Print(string(data))
	return nil
}
