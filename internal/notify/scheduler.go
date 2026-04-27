package notify

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/clock"
	"github.com/spk/spk-cockpit/internal/meeting"
)

// PopupHandler is invoked when a meeting hits its popup threshold. The handler
// should bring the main UI window forward and surface the meeting; the scheduler
// only fires the trigger and records the antifire mark.
type PopupHandler func(m api.Meeting)

// Scheduler scans for meetings due for notification or on-screen popup and fires
// each via its own channel. The two channels are independent: a meeting's DBus
// notification has its own time threshold (notify_min) and antifire (notified_at);
// the popup has its own (popup_min, popup_fired_at).
type Scheduler struct {
	meetings         *meeting.Service
	notifier         Notifier
	popup            PopupHandler
	clock            clock.Clock
	logger           *slog.Logger
	defaultNotifyMin int
	defaultPopupMin  int
	tick             time.Duration
}

// SchedulerConfig configures a Scheduler.
type SchedulerConfig struct {
	Meetings         *meeting.Service
	Notifier         Notifier
	Popup            PopupHandler
	Clock            clock.Clock
	Logger           *slog.Logger
	DefaultNotifyMin int
	DefaultPopupMin  int
	Tick             time.Duration
}

// NewScheduler constructs a Scheduler with sane defaults (notify=5min, popup=1min, 30s tick).
func NewScheduler(cfg SchedulerConfig) *Scheduler {
	if cfg.DefaultNotifyMin == 0 {
		cfg.DefaultNotifyMin = 5
	}
	if cfg.DefaultPopupMin == 0 {
		cfg.DefaultPopupMin = 1
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
		popup:            cfg.Popup,
		clock:            cfg.Clock,
		logger:           cfg.Logger,
		defaultNotifyMin: cfg.DefaultNotifyMin,
		defaultPopupMin:  cfg.DefaultPopupMin,
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
	s.scanNotify(ctx)
	s.scanPopup(ctx)
}

func (s *Scheduler) scanNotify(ctx context.Context) {
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

func (s *Scheduler) scanPopup(ctx context.Context) {
	if s.popup == nil {
		return
	}
	pending, err := s.meetings.PendingPopup(ctx, s.defaultPopupMin)
	if err != nil {
		s.logger.Warn("pending popup failed", "err", err)
		return
	}
	for _, m := range pending {
		s.popup(m)
		if err := s.meetings.MarkPopupFired(ctx, m.ID); err != nil {
			s.logger.Warn("mark popup fired failed", "id", m.ID, "err", err)
		}
	}
}
