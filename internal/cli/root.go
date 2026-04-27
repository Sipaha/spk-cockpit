// Package cli provides the command-line interface for spk-cockpit.
package cli

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:           "cockpit",
	Short:         "spk-cockpit — personal productivity tray app",
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root cobra command.
func Execute() error {
	return rootCmd.Execute()
}
