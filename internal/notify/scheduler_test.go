package notify_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/clock"
	"github.com/spk/spk-cockpit/internal/meeting"
	"github.com/spk/spk-cockpit/internal/meeting/fakerepo"
	"github.com/spk/spk-cockpit/internal/notify"
)

type captureNotifier struct {
	mu   sync.Mutex
	rows []string
}

func (c *captureNotifier) Notify(title, body string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rows = append(c.rows, title+"|"+body)
	return nil
}
func (c *captureNotifier) Close() error { return nil }
func (c *captureNotifier) snapshot() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.rows))
	copy(out, c.rows)
	return out
}

func TestScheduler_FiresOnceWhenWindowReached(t *testing.T) {
	t0 := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	c := clock.NewFake(t0)
	mrepo := fakerepo.NewMeeting()
	msvc := meeting.NewService(mrepo, c, nil)

	notifyMin := 5
	require.NoError(t, mrepo.Create(context.Background(), api.Meeting{
		ID: "m-1", Source: api.MeetingSourceManual, Title: "Standup",
		StartAt: t0.Add(4 * time.Minute).Unix(), EndAt: t0.Add(30 * time.Minute).Unix(),
		NotifyMin: &notifyMin, CreatedAt: t0.Unix(), UpdatedAt: t0.Unix(),
	}))

	captured := &captureNotifier{}
	sch := notify.NewScheduler(notify.SchedulerConfig{
		Meetings:         msvc,
		Notifier:         captured,
		Clock:            c,
		DefaultNotifyMin: 5,
		Tick:             10 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	go sch.Run(ctx)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if len(captured.snapshot()) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()

	got := captured.snapshot()
	require.Len(t, got, 1)
	require.Contains(t, got[0], "Standup")

	// Wait briefly to ensure no double-fire
	time.Sleep(50 * time.Millisecond)
	got2 := captured.snapshot()
	require.Equal(t, got, got2)
}
