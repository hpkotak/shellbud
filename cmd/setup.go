package cmd

import (
	"github.com/hpkotak/shellbud/internal/setup"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure ShellBud (first-time or reconfigure)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return setup.Run()
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
}
