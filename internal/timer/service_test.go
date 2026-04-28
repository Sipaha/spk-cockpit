package timer_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/spk/spk-cockpit/internal/clock"
	"github.com/spk/spk-cockpit/internal/timer"
	"github.com/spk/spk-cockpit/internal/timer/fakerepo"
)

func newTimerSvc(t *testing.T, t0 time.Time) (*timer.Service, *fakerepo.Timer) {
	t.Helper()
	r := fakerepo.NewTimer()
	c := clock.NewFake(t0)
	return timer.NewService(r, c, nil), r
}

func TestService_Start_AssignsAndReturnsSession(t *testing.T) {
	t0 := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	s, _ := newTimerSvc(t, t0)
	ctx := context.Background()

	got, err := s.Start(ctx, "todo-1")
	require.NoError(t, err)
	require.Equal(t, "todo-1", got.TodoID)
	require.Equal(t, t0.Unix(), got.StartedAt)
	require.Nil(t, got.EndedAt)
}

func TestService_Start_AllowsParallelTimers(t *testing.T) {
	t0 := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	s, r := newTimerSvc(t, t0)
	ctx := context.Background()

	first, err := s.Start(ctx, "todo-A")
	require.NoError(t, err)

	c := s.Clock().(*clock.Fake)
	c.Advance(5 * time.Minute)

	second, err := s.Start(ctx, "todo-B")
	require.NoError(t, err)
	require.Equal(t, "todo-B", second.TodoID)
	require.NotEqual(t, first.ID, second.ID)

	// Both sessions should be running concurrently — Start no longer auto-stops siblings.
	active, err := r.ListActive(ctx)
	require.NoError(t, err)
	require.Len(t, active, 2)
}

func TestService_Stop_ReturnsClosedSessionAndDuration(t *testing.T) {
	t0 := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	s, _ := newTimerSvc(t, t0)
	ctx := context.Background()

	_, err := s.Start(ctx, "todo-1")
	require.NoError(t, err)

	c := s.Clock().(*clock.Fake)
	c.Advance(90 * time.Second)

	stopped, dur, err := s.Stop(ctx, "todo-1")
	require.NoError(t, err)
	require.NotNil(t, stopped.EndedAt)
	require.Equal(t, int64(90), dur)
	require.Equal(t, "todo-1", stopped.TodoID)
}

func TestService_Stop_NoActiveReturnsErrNoActive(t *testing.T) {
	s, _ := newTimerSvc(t, time.Now())
	_, _, err := s.Stop(context.Background(), "todo-1")
	require.ErrorIs(t, err, timer.ErrNoActiveSession)
}

func TestService_Active_EmptyWhenNothingRunning(t *testing.T) {
	s, _ := newTimerSvc(t, time.Now())
	got, err := s.Active(context.Background())
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestService_TotalForTodo_AggregatesCompleted(t *testing.T) {
	t0 := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	s, _ := newTimerSvc(t, t0)
	ctx := context.Background()
	c := s.Clock().(*clock.Fake)

	_, _ = s.Start(ctx, "todo-1")
	c.Advance(60 * time.Second)
	_, _, _ = s.Stop(ctx, "todo-1")

	c.Advance(10 * time.Second)
	_, _ = s.Start(ctx, "todo-1")
	c.Advance(30 * time.Second)
	_, _, _ = s.Stop(ctx, "todo-1")

	total, err := s.TotalForTodo(ctx, "todo-1", 0)
	require.NoError(t, err)
	require.Equal(t, int64(90), total.TotalSec)
	require.Equal(t, 2, total.SessionCnt)
	require.False(t, total.HasActive)
}
