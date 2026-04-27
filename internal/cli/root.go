package cli

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:           "cockpit",
	Short:         "spk-cockpit — personal productivity tray app",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() error {
	return rootCmd.Execute()
}
