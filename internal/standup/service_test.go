package standup_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/clock"
	"github.com/spk/spk-cockpit/internal/eventbus"
	"github.com/spk/spk-cockpit/internal/standup"
	"github.com/spk/spk-cockpit/internal/sync/gitlab"
	"github.com/spk/spk-cockpit/internal/sync/tracker"
	"github.com/spk/spk-cockpit/internal/todo"
	"github.com/spk/spk-cockpit/internal/todo/fakerepo"
)

type stubEvents struct {
	events []api.TodoEvent
	err    error
}

func (s *stubEvents) ListAll(_ context.Context, _ int64, _ int) ([]api.TodoEvent, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.events, nil
}

func (s *stubEvents) Append(_ context.Context, e api.TodoEvent) error {
	s.events = append(s.events, e)
	return nil
}

func (s *stubEvents) ListByTodo(_ context.Context, _ string, _ int) ([]api.TodoEvent, error) {
	return nil, nil
}

func newTodoSvc(t *testing.T) (*todo.Service, *stubEvents) {
	t.Helper()
	repo := fakerepo.NewTodo()
	tags := fakerepo.NewTag()
	events := &stubEvents{}
	bus := eventbus.New()
	t.Cleanup(func() { bus.Close() })
	svc := todo.NewService(repo, tags, events, clock.Real(), bus)
	return svc, events
}

func TestService_Generate_TodosOnly(t *testing.T) {
	day := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	yStart := day.Truncate(24 * time.Hour).Add(-24 * time.Hour)

	svc, events := newTodoSvc(t)
	ctx := context.Background()

	// Yesterday: a todo that was completed yesterday.
	created, err := svc.Create(ctx, api.CreateTodoRequest{Title: "Ship feature X"})
	require.NoError(t, err)
	doneStatus := api.StatusDone
	_, _, err = svc.Update(ctx, created.ID, api.UpdateTodoRequest{Status: &doneStatus})
	require.NoError(t, err)

	// Override the event time to land in yesterday's window.
	for i := range events.events {
		if events.events[i].Kind == "status_changed" && events.events[i].ToValue == string(api.StatusDone) {
			events.events[i].At = yStart.Add(8 * time.Hour).Unix()
		}
	}

	// Today (in-progress).
	wip, err := svc.Create(ctx, api.CreateTodoRequest{Title: "Polish settings UI"})
	require.NoError(t, err)
	wipStatus := api.StatusInProgress
	_, _, err = svc.Update(ctx, wip.ID, api.UpdateTodoRequest{Status: &wipStatus})
	require.NoError(t, err)

	// Blocker: urgent overdue.
	dueOverdue := yStart.Add(-48 * time.Hour).Unix()
	_, err = svc.Create(ctx, api.CreateTodoRequest{Title: "Fix prod crash", Priority: api.PriorityUrgent, DueAt: &dueOverdue})
	require.NoError(t, err)

	s := standup.NewService(standup.Config{
		Todos:  svc,
		Events: events,
		Clock:  clock.Real(),
	})
	report, err := s.Generate(ctx, day)
	require.NoError(t, err)

	require.Equal(t, "2026-04-27", report.Day)
	require.Len(t, report.Yesterday, 1)
	require.Equal(t, "Ship feature X", report.Yesterday[0].Title)
	require.Equal(t, api.StandupSourceTodo, report.Yesterday[0].Source)

	require.GreaterOrEqual(t, len(report.Today), 1)
	foundWIP := false
	for _, it := range report.Today {
		if it.Title == "Polish settings UI" && it.Detail == "in progress" {
			foundWIP = true
		}
	}
	require.True(t, foundWIP, "expected WIP todo in Today")

	require.Len(t, report.Blockers, 1)
	require.Equal(t, "Fix prod crash", report.Blockers[0].Title)
	require.Equal(t, "overdue", report.Blockers[0].Detail)
}

func TestService_Generate_GitLabAndTrackerErrorsCollected(t *testing.T) {
	day := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	svc, events := newTodoSvc(t)

	gl := gitlab.NewFake()
	gl.SetError(errors.New("401"))
	tr := tracker.NewFake()
	tr.SetError(errors.New("500"))

	s := standup.NewService(standup.Config{
		Todos:        svc,
		Events:       events,
		GitLab:       gl,
		GitLabAuthor: "alice",
		Tracker:      tr,
		TrackerUser:  "alice",
		Clock:        clock.Real(),
	})
	report, err := s.Generate(context.Background(), day)
	require.NoError(t, err)
	require.Len(t, report.Errors, 2)
	require.Contains(t, report.Errors[0]+report.Errors[1], "gitlab")
	require.Contains(t, report.Errors[0]+report.Errors[1], "tracker")
}

func TestService_Generate_GitLabCommitsAppearInYesterday(t *testing.T) {
	day := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	svc, events := newTodoSvc(t)
	yStart := day.Truncate(24 * time.Hour).Add(-24 * time.Hour)

	gl := gitlab.NewFake()
	gl.SetCommits([]gitlab.Commit{{
		SHA: "abc123", Title: "feat: thing", Project: "team/x",
		URL: "https://gl/team/x/-/commit/abc123",
		At:  yStart.Add(10 * time.Hour),
	}})

	s := standup.NewService(standup.Config{
		Todos:        svc,
		Events:       events,
		GitLab:       gl,
		GitLabAuthor: "alice",
		Clock:        clock.Real(),
	})
	report, err := s.Generate(context.Background(), day)
	require.NoError(t, err)
	require.Len(t, report.Yesterday, 1)
	require.Equal(t, api.StandupSourceGitLab, report.Yesterday[0].Source)
	require.Equal(t, "feat: thing", report.Yesterday[0].Title)
}
