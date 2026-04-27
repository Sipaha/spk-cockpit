//go:build linux

package autostart

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// We test only the parts that don't require a real systemd-user session:
// unit-file path resolution and unit-file content composition.
func TestLinux_Path_RespectsXdgConfigHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	l, err := NewLinux()
	require.NoError(t, err)
	require.Equal(t, filepath.Join(tmp, "systemd", "user", "spk-cockpit.service"), l.Path())
}

func TestLinux_UnitTemplate_ContainsExePath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	l, err := NewLinux()
	require.NoError(t, err)

	// Write directly to skip systemctl invocation.
	require.NoError(t, os.MkdirAll(l.unitDir, 0o755))                        //nolint:gosec // test uses conventional systemd dir permissions
	require.NoError(t, os.WriteFile(l.Path(), []byte("placeholder"), 0o644)) //nolint:gosec // unit file is intentionally world-readable

	// Recreate via Install would try to invoke systemctl; instead, test the format string.
	content := unitTemplate
	require.True(t, strings.Contains(content, "ExecStart=%s start --foreground"))
	require.True(t, strings.Contains(content, "WantedBy=default.target"))
}
