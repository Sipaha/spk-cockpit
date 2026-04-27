package paths

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPaths_DefaultsRespectXDG(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(tmp, "state"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "config"))
	t.Setenv("SPK_COCKPIT_DATA_DIR", "")
	t.Setenv("SPK_COCKPIT_STATE_DIR", "")
	t.Setenv("SPK_COCKPIT_CONFIG_DIR", "")

	p, err := New()
	require.NoError(t, err)

	require.Equal(t, filepath.Join(tmp, "data", "spk-cockpit"), p.DataDir)
	require.Equal(t, filepath.Join(tmp, "state", "spk-cockpit"), p.StateDir)
	require.Equal(t, filepath.Join(tmp, "config", "spk-cockpit"), p.ConfigDir)
	require.Equal(t, filepath.Join(p.DataDir, "cockpit.db"), p.DBFile)
	require.Equal(t, filepath.Join(p.StateDir, "cockpit.sock"), p.SocketFile)

	for _, d := range []string{p.DataDir, p.StateDir, p.ConfigDir} {
		_, err := os.Stat(d)
		require.NoError(t, err, "directory %s should exist", d)
	}
}

func TestPaths_EnvOverridesXDG(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("SPK_COCKPIT_DATA_DIR", filepath.Join(tmp, "custom-data"))
	t.Setenv("SPK_COCKPIT_STATE_DIR", filepath.Join(tmp, "custom-state"))
	t.Setenv("SPK_COCKPIT_CONFIG_DIR", filepath.Join(tmp, "custom-config"))

	p, err := New()
	require.NoError(t, err)

	require.Equal(t, filepath.Join(tmp, "custom-data"), p.DataDir)
	require.Equal(t, filepath.Join(tmp, "custom-state"), p.StateDir)
	require.Equal(t, filepath.Join(tmp, "custom-config"), p.ConfigDir)
}
