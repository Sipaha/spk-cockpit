// Package cli provides the command-line interface for spk-cockpit.
package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "spk-cockpit",
	Short:         "spk-cockpit — personal productivity tray app",
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root cobra command with a SIGINT/SIGTERM-cancellable context.
func Execute() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return rootCmd.ExecuteContext(ctx)
}
