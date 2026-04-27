package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/spk/spk-cockpit/internal/paths"
	"github.com/spk/spk-cockpit/internal/window"
	webembed "github.com/spk/spk-cockpit/web/embed"
)

var popupFlags struct {
	meetingID string
	socket    string
}

var popupCmd = &cobra.Command{
	Use:   "popup",
	Short: "Open the meeting popup window (spawned by the daemon, normally not run by hand)",
	RunE: func(_ *cobra.Command, _ []string) error {
		if popupFlags.meetingID == "" {
			return errors.New("--meeting-id is required")
		}
		sock := popupFlags.socket
		if sock == "" {
			p, err := paths.New()
			if err != nil {
				return err
			}
			sock = p.SocketFile
		}
		return window.RunPopup(webembed.DistFS, sock, popupFlags.meetingID)
	},
}

func init() {
	popupCmd.Flags().StringVar(&popupFlags.meetingID, "meeting-id", "", "Meeting ID to render in the popup")
	popupCmd.Flags().StringVar(&popupFlags.socket, "socket", "", "Daemon UDS socket path (default: detected from paths)")
	rootCmd.AddCommand(popupCmd)
}
