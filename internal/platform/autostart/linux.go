//go:build linux

package autostart

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Linux implements Backend via systemd-user.
type Linux struct {
	unitDir  string // ~/.config/systemd/user
	unitFile string // spk-cockpit.service
}

// NewLinux constructs a Linux backend rooted at the user's $XDG_CONFIG_HOME or ~/.config.
func NewLinux() (*Linux, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	cfg := os.Getenv("XDG_CONFIG_HOME")
	if cfg == "" {
		cfg = filepath.Join(home, ".config")
	}
	return &Linux{
		unitDir:  filepath.Join(cfg, "systemd", "user"),
		unitFile: "spk-cockpit.service",
	}, nil
}

// Path returns the full path to the unit file (used for tests/inspection).
func (l *Linux) Path() string { return filepath.Join(l.unitDir, l.unitFile) }

const unitTemplate = `[Unit]
Description=spk-cockpit personal productivity tray
After=graphical-session.target

[Service]
ExecStart=%s start --foreground
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
`

// Install writes the unit file and runs systemctl --user daemon-reload + enable --now.
func (l *Linux) Install(exePath string) error {
	if err := os.MkdirAll(l.unitDir, 0o755); err != nil { //nolint:gosec // systemd unit dirs are conventionally world-readable
		return fmt.Errorf("mkdir unit dir: %w", err)
	}
	content := fmt.Sprintf(unitTemplate, exePath)
	if err := os.WriteFile(l.Path(), []byte(content), 0o644); err != nil { //nolint:gosec // unit file is intentionally world-readable
		return fmt.Errorf("write unit: %w", err)
	}
	if err := runSystemctl("daemon-reload"); err != nil {
		return err
	}
	if err := runSystemctl("enable", "--now", l.unitFile); err != nil {
		return err
	}
	return nil
}

// Uninstall disables, stops, and removes the unit file. Idempotent.
func (l *Linux) Uninstall() error {
	_ = runSystemctl("disable", "--now", l.unitFile) // ignore errors (may already be gone)
	if err := os.Remove(l.Path()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove unit: %w", err)
	}
	_ = runSystemctl("daemon-reload")
	return nil
}

// Status checks if the unit file exists and queries its enabled state.
func (l *Linux) Status() (Status, error) {
	if _, err := os.Stat(l.Path()); os.IsNotExist(err) {
		return Status{Installed: false}, nil
	} else if err != nil {
		return Status{}, err
	}
	st := Status{Installed: true}
	out, err := exec.Command("systemctl", "--user", "is-enabled", l.unitFile).CombinedOutput() //nolint:gosec // l.unitFile is a fixed constant
	st.Detail = string(out)
	if err == nil {
		st.Enabled = true
	}
	return st, nil
}

func runSystemctl(args ...string) error {
	cmd := exec.Command("systemctl", append([]string{"--user"}, args...)...) //nolint:gosec // args are controlled by internal callers only
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl %v: %w (%s)", args, err, string(out))
	}
	return nil
}
