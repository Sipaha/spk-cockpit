// Package cli provides the command-line interface for spk-cockpit.
package cli

import (
	"context"
	"io/fs"
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

// pkgFrontendFS holds the embedded React UI; main.go passes it to Execute,
// runStart_wails reads it when wiring the desktop runner. The non-wails
// runStart stub never touches it.
var pkgFrontendFS fs.FS

// Execute runs the root cobra command with a SIGINT/SIGTERM-cancellable context.
// frontendFS is the bundled React build supplied by main.go (cmd/cockpit/embed.go).
func Execute(frontendFS fs.FS) error {
	pkgFrontendFS = frontendFS
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return rootCmd.ExecuteContext(ctx)
}
