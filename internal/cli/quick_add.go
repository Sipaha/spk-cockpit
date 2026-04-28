package cli

import (
	"github.com/spf13/cobra"

	"github.com/spk/spk-cockpit/internal/paths"
	"github.com/spk/spk-cockpit/internal/window"
	webembed "github.com/spk/spk-cockpit/web/embed"
)

var quickAddFlags struct {
	socket string
}

var quickAddCmd = &cobra.Command{
	Use:   "quick-add",
	Short: "Open a small standalone window to add a todo (spawned by the daemon)",
	RunE: func(_ *cobra.Command, _ []string) error {
		sock := quickAddFlags.socket
		if sock == "" {
			p, err := paths.New()
			if err != nil {
				return err
			}
			sock = p.SocketFile
		}
		return window.RunQuickAdd(webembed.DistFS, sock)
	},
}

func init() {
	quickAddCmd.Flags().StringVar(&quickAddFlags.socket, "socket", "", "Daemon UDS socket path (default: detected from paths)")
	rootCmd.AddCommand(quickAddCmd)
}
