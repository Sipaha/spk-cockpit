package store

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigrate_AppliesOnFreshDB(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "test.db")
	s, err := Open(dsn)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	require.NoError(t, Migrate(s.DB))

	rows, err := s.DB.Query(`SELECT version FROM schema_migrations ORDER BY version`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	var versions []int
	for rows.Next() {
		var v int
		require.NoError(t, rows.Scan(&v))
		versions = append(versions, v)
	}
	require.Equal(t, []int{1, 2, 3, 4, 5}, versions)

	for _, table := range []string{"todos", "tags", "todo_tags", "todo_events", "kv", "timer_sessions", "meetings", "notes", "secrets", "sync_state"} {
		var n int
		err := s.DB.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n)
		require.NoError(t, err, "table %s missing", table)
	}
}

func TestMigrate_IsIdempotent(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "test.db")
	s, err := Open(dsn)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()
	require.NoError(t, Migrate(s.DB))
	require.NoError(t, Migrate(s.DB))
}
