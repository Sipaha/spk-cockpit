package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/spk/spk-cockpit/internal/paths"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running spk-cockpit daemon",
	RunE: func(_ *cobra.Command, _ []string) error {
		p, err := paths.New()
		if err != nil {
			return fmt.Errorf("paths: %w", err)
		}
		pidFile := filepath.Join(p.StateDir, "cockpit.pid")
		raw, err := os.ReadFile(pidFile) //nolint:gosec // path is constructed from paths.New(), not user input
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("daemon is not running")
		}
		if err != nil {
			return fmt.Errorf("read pid file: %w", err)
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
		if err != nil {
			return fmt.Errorf("parse pid: %w", err)
		}
		proc, err := os.FindProcess(pid)
		if err != nil {
			return fmt.Errorf("find process %d: %w", pid, err)
		}
		if err := proc.Signal(syscall.SIGTERM); err != nil {
			return fmt.Errorf("signal: %w", err)
		}
		for i := 0; i < 50; i++ {
			if proc.Signal(syscall.Signal(0)) != nil {
				fmt.Println("daemon stopped")
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		}
		return errors.New("daemon did not exit after 5s; consider SIGKILL manually")
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
