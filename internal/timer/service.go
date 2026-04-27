package timer

import (
	"context"
	"fmt"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/clock"
)

// EventPublisher publishes domain events. May be nil — service is nil-safe.
type EventPublisher interface {
	Publish(api.Event)
}

// Service is the time-tracking domain service.
type Service struct {
	repo  TimerRepo
	clock clock.Clock
	bus   EventPublisher
}

// NewService constructs the Service with the given repository, clock, and optional event bus.
func NewService(r TimerRepo, c clock.Clock, bus EventPublisher) *Service {
	return &Service{repo: r, clock: c, bus: bus}
}

// Clock exposes the injected clock (used by tests).
func (s *Service) Clock() clock.Clock { return s.clock }

func (s *Service) publish(t string, data any) {
	if s.bus == nil {
		return
	}
	s.bus.Publish(api.Event{Type: t, Data: data})
}

// Start begins tracking time on todoID. If a different timer is already running,
// it is stopped first (one-active-globally rule). If the same todo is already
// running, the existing session is returned unchanged (no-op).
func (s *Service) Start(ctx context.Context, todoID string) (api.TimerSession, error) {
	cur, err := s.repo.Active(ctx)
	if err != nil {
		return api.TimerSession{}, fmt.Errorf("active: %w", err)
	}
	if cur != nil {
		if cur.TodoID == todoID {
			return *cur, nil
		}
		if _, _, err := s.stopActive(ctx, *cur); err != nil {
			return api.TimerSession{}, err
		}
	}

	now := s.clock.Now().Unix()
	id, err := s.repo.Start(ctx, todoID, now, "manual")
	if err != nil {
		return api.TimerSession{}, fmt.Errorf("start: %w", err)
	}
	session := api.TimerSession{ID: id, TodoID: todoID, StartedAt: now, Source: "manual"}
	s.publish(api.EventTimerStarted, api.TimerStartedData{
		TodoID: todoID, SessionID: id, StartedAt: now,
	})
	return session, nil
}

// Stop ends the currently-active session, returning it and its duration in seconds.
// Returns ErrNoActiveSession if no timer is running.
func (s *Service) Stop(ctx context.Context) (api.TimerSession, int64, error) {
	cur, err := s.repo.Active(ctx)
	if err != nil {
		return api.TimerSession{}, 0, fmt.Errorf("active: %w", err)
	}
	if cur == nil {
		return api.TimerSession{}, 0, ErrNoActiveSession
	}
	return s.stopActive(ctx, *cur)
}

// stopActive closes an active session and emits EventTimerStopped.
func (s *Service) stopActive(ctx context.Context, cur api.TimerSession) (api.TimerSession, int64, error) {
	now := s.clock.Now().Unix()
	stopped, err := s.repo.Stop(ctx, cur.TodoID, now)
	if err != nil {
		return api.TimerSession{}, 0, fmt.Errorf("stop: %w", err)
	}
	dur := *stopped.EndedAt - stopped.StartedAt
	s.publish(api.EventTimerStopped, api.TimerStoppedData{
		TodoID: stopped.TodoID, SessionID: stopped.ID, EndedAt: *stopped.EndedAt, DurationSec: dur,
	})
	return stopped, dur, nil
}

// Active returns the currently-running session, or nil if no timer is active.
func (s *Service) Active(ctx context.Context) (*api.TimerSession, error) {
	return s.repo.Active(ctx)
}

// TotalForTodo aggregates completed sessions for todoID since sinceUnix and
// reports whether an active session is on that todo.
func (s *Service) TotalForTodo(ctx context.Context, todoID string, sinceUnix int64) (api.TodoTimeTotal, error) {
	total, cnt, err := s.repo.TotalForTodo(ctx, todoID, sinceUnix)
	if err != nil {
		return api.TodoTimeTotal{}, err
	}
	active, err := s.repo.Active(ctx)
	if err != nil {
		return api.TodoTimeTotal{}, err
	}
	hasActive := active != nil && active.TodoID == todoID
	return api.TodoTimeTotal{
		TodoID:     todoID,
		SinceUnix:  sinceUnix,
		TotalSec:   total,
		SessionCnt: cnt,
		HasActive:  hasActive,
	}, nil
}

// ListSessions returns recent sessions for a todo, newest first.
func (s *Service) ListSessions(ctx context.Context, todoID string, limit int) ([]api.TimerSession, error) {
	return s.repo.ListByTodo(ctx, todoID, limit)
}
