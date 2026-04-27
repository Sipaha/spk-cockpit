package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/todo"
	"github.com/spk/spk-cockpit/internal/todo/fakerepo"
)

type todoRepoCase struct {
	name string
	new  func(t *testing.T) todo.TodoRepo
}

func todoRepoCases() []todoRepoCase {
	return []todoRepoCase{
		{
			name: "fake",
			new:  func(_ *testing.T) todo.TodoRepo { return fakerepo.NewTodo() },
		},
		{
			name: "sqlite",
			new: func(t *testing.T) todo.TodoRepo {
				dsn := "file:" + filepath.Join(t.TempDir(), "t.db")
				s, err := Open(dsn)
				require.NoError(t, err)
				t.Cleanup(func() { _ = s.Close() })
				require.NoError(t, Migrate(s.DB))
				return NewTodoRepo(s.DB)
			},
		},
	}
}

func TestTodoRepo_Conformance(t *testing.T) {
	for _, c := range todoRepoCases() {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			r := c.new(t)

			td := api.Todo{
				ID:        "01H000000000000000000A0001",
				Title:     "Hello",
				Priority:  api.PriorityNormal,
				Status:    api.StatusOpen,
				CreatedAt: 100,
				UpdatedAt: 100,
			}
			require.NoError(t, r.Create(ctx, td))

			got, err := r.Get(ctx, td.ID)
			require.NoError(t, err)
			require.Equal(t, td.Title, got.Title)

			_, err = r.Update(ctx, td.ID, func(x *api.Todo) error {
				x.Title = "Updated"
				x.UpdatedAt = 200
				return nil
			})
			require.NoError(t, err)
			got, err = r.Get(ctx, td.ID)
			require.NoError(t, err)
			require.Equal(t, "Updated", got.Title)

			list, err := r.List(ctx, todo.TodoFilter{})
			require.NoError(t, err)
			require.Len(t, list, 1)

			require.NoError(t, r.Delete(ctx, td.ID))
			_, err = r.Get(ctx, td.ID)
			require.ErrorIs(t, err, todo.ErrNotFound)
		})
	}
}
