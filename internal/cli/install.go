//go:build linux

package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/spk/spk-cockpit/internal/platform/autostart"
)

var installFlags struct {
	autostart bool
	uninstall bool
}

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install autostart and other OS-level integrations",
	RunE: func(_ *cobra.Command, _ []string) error {
		if !installFlags.autostart && !installFlags.uninstall {
			return fmt.Errorf("specify --autostart to install or --uninstall to remove")
		}
		be, err := autostart.NewLinux()
		if err != nil {
			return err
		}

		if installFlags.uninstall {
			if err := be.Uninstall(); err != nil {
				return err
			}
			fmt.Println("autostart removed.")
			return nil
		}

		exePath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("locate executable: %w", err)
		}
		if err := be.Install(exePath); err != nil {
			return err
		}
		fmt.Println("autostart installed and enabled.")
		fmt.Printf("Unit: %s\n", exePath)
		return nil
	},
}

func init() {
	installCmd.Flags().BoolVar(&installFlags.autostart, "autostart", false, "Install the systemd-user autostart unit")
	installCmd.Flags().BoolVar(&installFlags.uninstall, "uninstall", false, "Remove the autostart unit")
	rootCmd.AddCommand(installCmd)
}
