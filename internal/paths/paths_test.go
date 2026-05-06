package paths

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPaths_DefaultsToSpkRoot(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("SPK_COCKPIT_DATA_DIR", "")
	t.Setenv("SPK_COCKPIT_STATE_DIR", "")
	t.Setenv("SPK_COCKPIT_CONFIG_DIR", "")
	t.Setenv("SPK_COCKPIT_LOG_DIR", "")

	p, err := New()
	require.NoError(t, err)

	root := filepath.Join(tmp, ".spk", "spk-cockpit")
	require.Equal(t, filepath.Join(root, "data"), p.DataDir)
	require.Equal(t, filepath.Join(root, "state"), p.StateDir)
	require.Equal(t, filepath.Join(root, "config"), p.ConfigDir)
	require.Equal(t, filepath.Join(root, "logs"), p.LogDir)
	require.Equal(t, filepath.Join(p.DataDir, "cockpit.db"), p.DBFile)
	require.Equal(t, filepath.Join(p.StateDir, "cockpit.sock"), p.SocketFile)
	require.Equal(t, filepath.Join(p.LogDir, "cockpit.log"), p.LogFile)

	for _, d := range []string{p.DataDir, p.StateDir, p.ConfigDir, p.LogDir} {
		_, err := os.Stat(d)
		require.NoError(t, err, "directory %s should exist", d)
	}
}

func TestPaths_EnvOverridesDefaults(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("SPK_COCKPIT_DATA_DIR", filepath.Join(tmp, "custom-data"))
	t.Setenv("SPK_COCKPIT_STATE_DIR", filepath.Join(tmp, "custom-state"))
	t.Setenv("SPK_COCKPIT_CONFIG_DIR", filepath.Join(tmp, "custom-config"))
	t.Setenv("SPK_COCKPIT_LOG_DIR", filepath.Join(tmp, "custom-logs"))

	p, err := New()
	require.NoError(t, err)

	require.Equal(t, filepath.Join(tmp, "custom-data"), p.DataDir)
	require.Equal(t, filepath.Join(tmp, "custom-state"), p.StateDir)
	require.Equal(t, filepath.Join(tmp, "custom-config"), p.ConfigDir)
	require.Equal(t, filepath.Join(tmp, "custom-logs"), p.LogDir)
}
