package todo_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/clock"
	"github.com/spk/spk-cockpit/internal/todo"
	"github.com/spk/spk-cockpit/internal/todo/fakerepo"
)

//nolint:unparam // tr (*fakerepo.Todo) is returned for completeness; callers may need it in future tests
func newSvc(t *testing.T, now time.Time) (*todo.Service, *fakerepo.Todo, *fakerepo.Tag, *fakerepo.Event) {
	t.Helper()
	tr, gr, er := fakerepo.NewTodo(), fakerepo.NewTag(), fakerepo.NewEvent()
	c := clock.NewFake(now)
	return todo.NewService(tr, gr, er, c, nil), tr, gr, er
}

func TestService_Create_AssignsIDAndTimestamps(t *testing.T) {
	now := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	s, _, _, er := newSvc(t, now)

	got, err := s.Create(context.Background(), api.CreateTodoRequest{
		Title:    "Buy milk",
		Priority: api.PriorityNormal,
	})
	require.NoError(t, err)
	require.NotEmpty(t, got.ID)
	require.Equal(t, "Buy milk", got.Title)
	require.Equal(t, api.StatusOpen, got.Status)
	require.Equal(t, now.Unix(), got.CreatedAt)
	require.Equal(t, now.Unix(), got.UpdatedAt)
	require.Nil(t, got.DoneAt)

	events, err := er.ListByTodo(context.Background(), got.ID, 0)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, "created", events[0].Kind)
}

func TestService_Create_RejectsEmptyTitle(t *testing.T) {
	s, _, _, _ := newSvc(t, time.Now())
	_, err := s.Create(context.Background(), api.CreateTodoRequest{Title: "  "})
	require.Error(t, err)
}

func TestService_Get_LoadsTags(t *testing.T) {
	now := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	s, _, _, _ := newSvc(t, now)
	got, err := s.Create(context.Background(), api.CreateTodoRequest{Title: "X", Tags: []string{"backend", "urgent"}})
	require.NoError(t, err)

	loaded, err := s.Get(context.Background(), got.ID)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"backend", "urgent"}, loaded.Tags)
}

func TestService_Update_StatusChangeEmitsEventAndDoneAt(t *testing.T) {
	t0 := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	s, _, _, er := newSvc(t, t0)
	got, err := s.Create(context.Background(), api.CreateTodoRequest{Title: "X"})
	require.NoError(t, err)

	done := api.StatusDone
	updated, _, err := s.Update(context.Background(), got.ID, api.UpdateTodoRequest{Status: &done})
	require.NoError(t, err)
	require.Equal(t, api.StatusDone, updated.Status)
	require.NotNil(t, updated.DoneAt)

	events, err := er.ListByTodo(context.Background(), got.ID, 0)
	require.NoError(t, err)
	kinds := []string{}
	for _, e := range events {
		kinds = append(kinds, e.Kind)
	}
	require.Contains(t, kinds, "status_changed")
}

func TestService_Update_PriorityChangeEmitsEvent(t *testing.T) {
	s, _, _, er := newSvc(t, time.Now())
	got, err := s.Create(context.Background(), api.CreateTodoRequest{Title: "X", Priority: api.PriorityNormal})
	require.NoError(t, err)

	urgent := api.PriorityUrgent
	_, _, err = s.Update(context.Background(), got.ID, api.UpdateTodoRequest{Priority: &urgent})
	require.NoError(t, err)

	events, err := er.ListByTodo(context.Background(), got.ID, 0)
	require.NoError(t, err)
	kinds := []string{}
	for _, e := range events {
		kinds = append(kinds, e.Kind)
	}
	require.Contains(t, kinds, "priority_changed")
}

func TestService_Delete_AppendsEventAndHidesFromGet(t *testing.T) {
	s, _, _, er := newSvc(t, time.Now())
	got, err := s.Create(context.Background(), api.CreateTodoRequest{Title: "X"})
	require.NoError(t, err)

	require.NoError(t, s.Delete(context.Background(), got.ID))

	_, err = s.Get(context.Background(), got.ID)
	require.ErrorIs(t, err, todo.ErrNotFound)

	events, err := er.ListByTodo(context.Background(), got.ID, 0)
	require.NoError(t, err)
	last := events[0]
	require.Equal(t, "deleted", last.Kind)
}

func TestService_List_FiltersAndExcludesDoneByDefault(t *testing.T) {
	s, _, _, _ := newSvc(t, time.Now())
	a, _ := s.Create(context.Background(), api.CreateTodoRequest{Title: "Open"})
	b, _ := s.Create(context.Background(), api.CreateTodoRequest{Title: "Done"})
	done := api.StatusDone
	_, _, _ = s.Update(context.Background(), b.ID, api.UpdateTodoRequest{Status: &done})

	got, err := s.List(context.Background(), todo.TodoFilter{})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, a.ID, got[0].ID)

	got, err = s.List(context.Background(), todo.TodoFilter{IncludeDone: true})
	require.NoError(t, err)
	require.Len(t, got, 2)
}
