// Package paths resolves XDG-compliant filesystem locations for spk-cockpit,
// with override via SPK_COCKPIT_* env vars (used by tests and packaging).
package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

// Paths holds all resolved filesystem paths for spk-cockpit.
type Paths struct {
	DataDir    string
	StateDir   string
	ConfigDir  string
	DBFile     string
	SocketFile string
	LogDir     string
	LogFile    string
}

// New resolves all paths and ensures the directories exist.
func New() (*Paths, error) {
	dataDir, err := resolve("SPK_COCKPIT_DATA_DIR", "XDG_DATA_HOME", ".local/share", "spk-cockpit")
	if err != nil {
		return nil, err
	}
	stateDir, err := resolve("SPK_COCKPIT_STATE_DIR", "XDG_STATE_HOME", ".local/state", "spk-cockpit")
	if err != nil {
		return nil, err
	}
	configDir, err := resolve("SPK_COCKPIT_CONFIG_DIR", "XDG_CONFIG_HOME", ".config", "spk-cockpit")
	if err != nil {
		return nil, err
	}

	logDir := filepath.Join(stateDir, "log")
	for _, d := range []string{dataDir, stateDir, configDir, logDir} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return nil, fmt.Errorf("mkdir %s: %w", d, err)
		}
	}

	return &Paths{
		DataDir:    dataDir,
		StateDir:   stateDir,
		ConfigDir:  configDir,
		LogDir:     logDir,
		DBFile:     filepath.Join(dataDir, "cockpit.db"),
		SocketFile: filepath.Join(stateDir, "cockpit.sock"),
		LogFile:    filepath.Join(logDir, "cockpit.log"),
	}, nil
}

func resolve(envVar, xdgVar, defaultRel, app string) (string, error) {
	if v := os.Getenv(envVar); v != "" {
		return v, nil
	}
	if v := os.Getenv(xdgVar); v != "" {
		return filepath.Join(v, app), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home: %w", err)
	}
	return filepath.Join(home, defaultRel, app), nil
}
