package notify

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/spk/spk-cockpit/internal/clock"
	"github.com/spk/spk-cockpit/internal/meeting"
)

// Scheduler scans for meetings due for notification and fires them via Notifier.
type Scheduler struct {
	meetings         *meeting.Service
	notifier         Notifier
	clock            clock.Clock
	logger           *slog.Logger
	defaultNotifyMin int
	tick             time.Duration
}

// SchedulerConfig configures a Scheduler.
type SchedulerConfig struct {
	Meetings         *meeting.Service
	Notifier         Notifier
	Clock            clock.Clock
	Logger           *slog.Logger
	DefaultNotifyMin int
	Tick             time.Duration
}

// NewScheduler constructs a Scheduler with sane defaults (5 min, 30s tick).
func NewScheduler(cfg SchedulerConfig) *Scheduler {
	if cfg.DefaultNotifyMin == 0 {
		cfg.DefaultNotifyMin = 5
	}
	if cfg.Tick == 0 {
		cfg.Tick = 30 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Scheduler{
		meetings:         cfg.Meetings,
		notifier:         cfg.Notifier,
		clock:            cfg.Clock,
		logger:           cfg.Logger,
		defaultNotifyMin: cfg.DefaultNotifyMin,
		tick:             cfg.Tick,
	}
}

// Run blocks until ctx is done, scanning every tick.
func (s *Scheduler) Run(ctx context.Context) {
	t := time.NewTicker(s.tick)
	defer t.Stop()
	s.scanOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.scanOnce(ctx)
		}
	}
}

func (s *Scheduler) scanOnce(ctx context.Context) {
	pending, err := s.meetings.PendingNotification(ctx, s.defaultNotifyMin)
	if err != nil {
		s.logger.Warn("pending notify failed", "err", err)
		return
	}
	for _, m := range pending {
		body := fmt.Sprintf("Starts in %s", time.Until(time.Unix(m.StartAt, 0)).Round(time.Minute))
		if err := s.notifier.Notify(m.Title, body); err != nil {
			s.logger.Warn("notify failed", "id", m.ID, "err", err)
			continue
		}
		if err := s.meetings.MarkNotified(ctx, m.ID); err != nil {
			s.logger.Warn("mark notified failed", "id", m.ID, "err", err)
		}
	}
}
