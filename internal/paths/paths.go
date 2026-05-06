// Package paths resolves filesystem locations for spk-cockpit under
// ~/.spk/spk-cockpit/, with per-directory overrides via SPK_COCKPIT_*_DIR
// env vars (used by tests and packaging).
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
	LogDir     string
	DBFile     string
	SocketFile string
	LogFile    string
}

// New resolves all paths and ensures the directories exist.
func New() (*Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("user home: %w", err)
	}
	root := filepath.Join(home, ".spk", "spk-cockpit")

	dataDir := override("SPK_COCKPIT_DATA_DIR", filepath.Join(root, "data"))
	stateDir := override("SPK_COCKPIT_STATE_DIR", filepath.Join(root, "state"))
	configDir := override("SPK_COCKPIT_CONFIG_DIR", filepath.Join(root, "config"))
	logDir := override("SPK_COCKPIT_LOG_DIR", filepath.Join(root, "logs"))

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

func override(envVar, defaultPath string) string {
	if v := os.Getenv(envVar); v != "" {
		return v
	}
	return defaultPath
}
