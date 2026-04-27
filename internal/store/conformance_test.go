package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/timer"
	timerfake "github.com/spk/spk-cockpit/internal/timer/fakerepo"
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

type timerRepoCase struct {
	name string
	new  func(t *testing.T) timer.TimerRepo
}

func timerRepoCases() []timerRepoCase {
	return []timerRepoCase{
		{
			name: "fake",
			new:  func(_ *testing.T) timer.TimerRepo { return timerfake.NewTimer() },
		},
		{
			name: "sqlite",
			new: func(t *testing.T) timer.TimerRepo {
				dsn := "file:" + filepath.Join(t.TempDir(), "t.db")
				s, err := Open(dsn)
				require.NoError(t, err)
				t.Cleanup(func() { _ = s.Close() })
				require.NoError(t, Migrate(s.DB))
				return NewTimerRepo(s.DB)
			},
		},
	}
}

func TestTimerRepo_Conformance(t *testing.T) {
	for _, c := range timerRepoCases() {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			r := c.new(t)

			active, err := r.Active(ctx)
			require.NoError(t, err)
			require.Nil(t, active)

			id, err := r.Start(ctx, "todo-1", 100, "manual")
			require.NoError(t, err)
			require.NotZero(t, id)

			active, err = r.Active(ctx)
			require.NoError(t, err)
			require.NotNil(t, active)
			require.Equal(t, "todo-1", active.TodoID)
			require.Nil(t, active.EndedAt)

			// Cannot start twice on same todo without stopping.
			_, err = r.Start(ctx, "todo-1", 110, "manual")
			require.ErrorIs(t, err, timer.ErrAlreadyActiveOnTodo)

			s, err := r.Stop(ctx, "todo-1", 160)
			require.NoError(t, err)
			require.NotNil(t, s.EndedAt)
			require.Equal(t, int64(160), *s.EndedAt)

			active, err = r.Active(ctx)
			require.NoError(t, err)
			require.Nil(t, active)

			total, cnt, err := r.TotalForTodo(ctx, "todo-1", 0)
			require.NoError(t, err)
			require.Equal(t, int64(60), total)
			require.Equal(t, 1, cnt)

			// Stop with nothing running -> error.
			_, err = r.Stop(ctx, "todo-1", 200)
			require.ErrorIs(t, err, timer.ErrNoActiveSession)
		})
	}
}
