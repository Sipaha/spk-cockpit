package caldav

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/clock"
	"github.com/spk/spk-cockpit/internal/meeting"
)

// SourceName is the sync_state key for CalDAV.
const SourceName = "caldav"

// Syncer is a periodic CalDAV worker that upserts meetings from any RFC 4791-compliant server.
type Syncer struct {
	client    Client
	meetings  *meeting.Service
	state     meeting.SyncStateRepo
	clock     clock.Clock
	logger    *slog.Logger
	bus       api.EventPublisher
	interval  time.Duration
	rangeBack time.Duration
	rangeFwd  time.Duration

	mu       sync.Mutex
	trigger  chan struct{}
	running  bool
	lastErr  string
	lastOkAt *int64
}

// SyncerConfig configures a Syncer.
type SyncerConfig struct {
	Client       Client
	Meetings     *meeting.Service
	State        meeting.SyncStateRepo
	Clock        clock.Clock
	Logger       *slog.Logger
	Bus          api.EventPublisher
	Interval     time.Duration
	RangeBack    time.Duration
	RangeForward time.Duration
}

// NewSyncer constructs a Syncer with sane defaults (5m interval, -7d / +30d range).
func NewSyncer(cfg SyncerConfig) *Syncer {
	if cfg.Interval == 0 {
		cfg.Interval = 5 * time.Minute
	}
	if cfg.RangeBack == 0 {
		cfg.RangeBack = 7 * 24 * time.Hour
	}
	if cfg.RangeForward == 0 {
		cfg.RangeForward = 30 * 24 * time.Hour
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Syncer{
		client:    cfg.Client,
		meetings:  cfg.Meetings,
		state:     cfg.State,
		clock:     cfg.Clock,
		logger:    cfg.Logger,
		bus:       cfg.Bus,
		interval:  cfg.Interval,
		rangeBack: cfg.RangeBack,
		rangeFwd:  cfg.RangeForward,
		trigger:   make(chan struct{}, 1),
	}
}

// Run blocks until ctx is done. It runs once immediately, then every Interval, plus on TriggerNow.
func (s *Syncer) Run(ctx context.Context) {
	tick := time.NewTicker(s.interval)
	defer tick.Stop()
	s.runOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			s.runOnce(ctx)
		case <-s.trigger:
			s.runOnce(ctx)
		}
	}
}

// TriggerNow asks the syncer to run immediately (non-blocking).
func (s *Syncer) TriggerNow(source string) error {
	if source != SourceName {
		return fmt.Errorf("unknown source %q", source)
	}
	select {
	case s.trigger <- struct{}{}:
	default:
	}
	return nil
}

// Status reports the syncer's current state.
func (s *Syncer) Status() []api.SyncStateEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return []api.SyncStateEntry{{Source: SourceName, LastErr: s.lastErr, LastOkAt: s.lastOkAt}}
}

func (s *Syncer) runOnce(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	now := s.clock.Now()
	from := now.Add(-s.rangeBack)
	to := now.Add(s.rangeFwd)

	prev, _ := s.state.Get(ctx, SourceName)
	events, newCTag, unchanged, err := s.client.FetchEvents(ctx, from, to, prev.Cursor)
	if err != nil {
		s.recordErr(ctx, err)
		return
	}
	if unchanged {
		s.recordOK(ctx, prev.Cursor)
		return
	}

	seen := map[string]bool{}
	for _, ev := range events {
		seen[ev.ExternalUID] = true
		if _, err := s.meetings.UpsertFromCalDAV(ctx, ev); err != nil {
			s.logger.Warn("upsert meeting failed", "uid", ev.ExternalUID, "err", err)
		}
	}

	existing, err := s.meetings.List(ctx, meeting.MeetingFilter{FromUnix: from.Unix(), ToUnix: to.Unix(), IncludeDone: true})
	if err == nil {
		for _, m := range existing {
			if m.Source != api.MeetingSourceCalDAV {
				continue
			}
			if !seen[m.ExternalUID] && !m.Cancelled {
				if err := s.meetings.MarkExternalCancelled(ctx, m.ExternalUID); err != nil {
					s.logger.Warn("cancel meeting failed", "uid", m.ExternalUID, "err", err)
				}
			}
		}
	}

	s.recordOK(ctx, newCTag)
}

func (s *Syncer) recordErr(ctx context.Context, err error) {
	s.mu.Lock()
	s.lastErr = err.Error()
	s.mu.Unlock()
	_ = s.state.Save(ctx, api.SyncStateEntry{Source: SourceName, LastErr: err.Error()})
	if s.bus != nil {
		s.bus.Publish(api.Event{
			Type: api.EventSyncStateChanged,
			Data: api.SyncStateChangedData{Source: SourceName, Status: "failed", LastErr: err.Error()},
		})
	}
	s.logger.Warn("caldav sync failed", "err", err)
}

func (s *Syncer) recordOK(ctx context.Context, cursor string) {
	now := s.clock.Now().Unix()
	s.mu.Lock()
	s.lastErr = ""
	s.lastOkAt = &now
	s.mu.Unlock()
	_ = s.state.Save(ctx, api.SyncStateEntry{Source: SourceName, Cursor: cursor, LastOkAt: &now})
	if s.bus != nil {
		s.bus.Publish(api.Event{
			Type: api.EventSyncStateChanged,
			Data: api.SyncStateChangedData{Source: SourceName, Status: "ok"},
		})
	}
}
