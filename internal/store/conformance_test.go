package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/meeting"
	meetingfake "github.com/spk/spk-cockpit/internal/meeting/fakerepo"
	"github.com/spk/spk-cockpit/internal/note"
	notefake "github.com/spk/spk-cockpit/internal/note/fakerepo"
	"github.com/spk/spk-cockpit/internal/secret"
	secretfake "github.com/spk/spk-cockpit/internal/secret/fakerepo"
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

type meetingRepoCase struct {
	name string
	new  func(t *testing.T) meeting.MeetingRepo
}

func meetingRepoCases() []meetingRepoCase {
	return []meetingRepoCase{
		{"fake", func(_ *testing.T) meeting.MeetingRepo { return meetingfake.NewMeeting() }},
		{"sqlite", func(t *testing.T) meeting.MeetingRepo {
			dsn := "file:" + filepath.Join(t.TempDir(), "t.db")
			s, err := Open(dsn)
			require.NoError(t, err)
			t.Cleanup(func() { _ = s.Close() })
			require.NoError(t, Migrate(s.DB))
			return NewMeetingRepo(s.DB)
		}},
	}
}

func TestMeetingRepo_Conformance(t *testing.T) {
	for _, c := range meetingRepoCases() {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			r := c.new(t)

			m := api.Meeting{
				ID: "m-1", Source: api.MeetingSourceManual,
				Title: "Hello", StartAt: 1000, EndAt: 1500,
				CreatedAt: 100, UpdatedAt: 100,
			}
			require.NoError(t, r.Create(ctx, m))
			got, err := r.Get(ctx, "m-1")
			require.NoError(t, err)
			require.Equal(t, "Hello", got.Title)

			// UpsertExternal — insert
			ext := api.Meeting{
				ID: "m-2", Source: api.MeetingSourceCalDAV, ExternalUID: "uid-1", ExternalETag: "etag-1",
				Title: "External", StartAt: 2000, EndAt: 2500,
				CreatedAt: 100, UpdatedAt: 100,
			}
			ins, inserted, err := r.UpsertExternal(ctx, ext)
			require.NoError(t, err)
			require.True(t, inserted)
			require.Equal(t, "External", ins.Title)

			// UpsertExternal — update with same start_at: notified_at preserved.
			require.NoError(t, r.MarkNotified(ctx, ins.ID, 1900))
			ext2 := ext
			ext2.Title = "External v2"
			ext2.UpdatedAt = 200
			updated, inserted, err := r.UpsertExternal(ctx, ext2)
			require.NoError(t, err)
			require.False(t, inserted)
			require.Equal(t, "External v2", updated.Title)
			require.NotNil(t, updated.NotifiedAt)

			// UpsertExternal — update with NEW start_at: notified_at reset.
			ext3 := ext2
			ext3.StartAt = 3000
			ext3.UpdatedAt = 300
			rescheduled, _, err := r.UpsertExternal(ctx, ext3)
			require.NoError(t, err)
			require.Nil(t, rescheduled.NotifiedAt)

			// PendingNotification at t=2950, default 5 min: ext3 starts at 3000, notify_min default 5 → 3000 - 300 = 2700 ≤ 2950 → match.
			pending, err := r.PendingNotification(ctx, 2950, 5)
			require.NoError(t, err)
			require.Len(t, pending, 1)
			require.Equal(t, "m-2", pending[0].ID)

			// MarkNotified hides it.
			require.NoError(t, r.MarkNotified(ctx, "m-2", 2950))
			pending, err = r.PendingNotification(ctx, 2950, 5)
			require.NoError(t, err)
			require.Len(t, pending, 0)

			// Delete
			require.NoError(t, r.Delete(ctx, "m-1"))
			_, err = r.Get(ctx, "m-1")
			require.ErrorIs(t, err, meeting.ErrNotFound)
		})
	}
}

type noteRepoCase struct {
	name string
	new  func(t *testing.T) note.NoteRepo
}

func noteRepoCases() []noteRepoCase {
	return []noteRepoCase{
		{"fake", func(_ *testing.T) note.NoteRepo { return notefake.NewNote() }},
		{"sqlite", func(t *testing.T) note.NoteRepo {
			dsn := "file:" + filepath.Join(t.TempDir(), "t.db")
			s, err := Open(dsn)
			require.NoError(t, err)
			t.Cleanup(func() { _ = s.Close() })
			require.NoError(t, Migrate(s.DB))
			return NewNoteRepo(s.DB)
		}},
	}
}

func TestNoteRepo_Conformance(t *testing.T) {
	for _, c := range noteRepoCases() {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			r := c.new(t)

			n := api.Note{ID: "n-1", MeetingID: "m-1", Body: "hello", CreatedAt: 100, UpdatedAt: 100}
			require.NoError(t, r.Upsert(ctx, n))
			got, err := r.Get(ctx, "n-1")
			require.NoError(t, err)
			require.Equal(t, "hello", got.Body)

			n.Body = "world"
			n.UpdatedAt = 200
			require.NoError(t, r.Upsert(ctx, n))

			byMeeting, err := r.FindByAttachment(ctx, "m-1", "")
			require.NoError(t, err)
			require.Equal(t, "world", byMeeting.Body)

			require.NoError(t, r.Delete(ctx, "n-1"))
			_, err = r.Get(ctx, "n-1")
			require.ErrorIs(t, err, note.ErrNotFound)
		})
	}
}

type secretRepoCase struct {
	name string
	new  func(t *testing.T) secret.SecretRepo
}

func secretRepoCases() []secretRepoCase {
	return []secretRepoCase{
		{"fake", func(_ *testing.T) secret.SecretRepo { return secretfake.NewSecret() }},
		{"sqlite", func(t *testing.T) secret.SecretRepo {
			dsn := "file:" + filepath.Join(t.TempDir(), "t.db")
			s, err := Open(dsn)
			require.NoError(t, err)
			t.Cleanup(func() { _ = s.Close() })
			require.NoError(t, Migrate(s.DB))
			return NewSecretRepo(s.DB)
		}},
	}
}

func TestSecretRepo_Conformance(t *testing.T) {
	for _, c := range secretRepoCases() {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			r := c.new(t)

			s := secret.EncryptedSecret{Name: "yandex_caldav", Ciphertext: []byte("ct"), Nonce: []byte("nn"), UpdatedAt: 100}
			require.NoError(t, r.Set(ctx, s))
			got, err := r.Get(ctx, "yandex_caldav")
			require.NoError(t, err)
			require.Equal(t, []byte("ct"), got.Ciphertext)

			names, err := r.ListNames(ctx)
			require.NoError(t, err)
			require.Equal(t, []string{"yandex_caldav"}, names)

			require.NoError(t, r.Delete(ctx, "yandex_caldav"))
			_, err = r.Get(ctx, "yandex_caldav")
			require.ErrorIs(t, err, secret.ErrNotFound)
		})
	}
}
