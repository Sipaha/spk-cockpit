package cli

import "github.com/spf13/cobra"

func init() {
	rootCmd.RunE = func(cmd *cobra.Command, _ []string) error {
		return runStart(cmd.Context())
	}
}
