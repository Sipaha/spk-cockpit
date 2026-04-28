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

// Start begins tracking time on todoID. Multiple todos may have active timers
// concurrently — the partial unique index (todo_id WHERE ended_at IS NULL)
// only forbids duplicates on the same todo, so calling Start with a todoID
// that already has an active session is idempotent (returns the existing one).
func (s *Service) Start(ctx context.Context, todoID string) (api.TimerSession, error) {
	if cur, err := s.repo.ActiveByTodo(ctx, todoID); err != nil {
		return api.TimerSession{}, fmt.Errorf("active: %w", err)
	} else if cur != nil {
		return *cur, nil
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

// Stop ends the active session for todoID. Returns ErrNoActiveSession if that
// todo has no running timer. Other todos' timers are left alone.
func (s *Service) Stop(ctx context.Context, todoID string) (api.TimerSession, int64, error) {
	cur, err := s.repo.ActiveByTodo(ctx, todoID)
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

// Active returns every currently-running session.
func (s *Service) Active(ctx context.Context) ([]api.TimerSession, error) {
	list, err := s.repo.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []api.TimerSession{}
	}
	return list, nil
}

// TotalForTodo aggregates completed sessions for todoID since sinceUnix and
// reports whether that specific todo has an active session.
func (s *Service) TotalForTodo(ctx context.Context, todoID string, sinceUnix int64) (api.TodoTimeTotal, error) {
	total, cnt, err := s.repo.TotalForTodo(ctx, todoID, sinceUnix)
	if err != nil {
		return api.TodoTimeTotal{}, err
	}
	active, err := s.repo.ActiveByTodo(ctx, todoID)
	if err != nil {
		return api.TodoTimeTotal{}, err
	}
	return api.TodoTimeTotal{
		TodoID:     todoID,
		SinceUnix:  sinceUnix,
		TotalSec:   total,
		SessionCnt: cnt,
		HasActive:  active != nil,
	}, nil
}

// ListSessions returns recent sessions for a todo, newest first.
func (s *Service) ListSessions(ctx context.Context, todoID string, limit int) ([]api.TimerSession, error) {
	return s.repo.ListByTodo(ctx, todoID, limit)
}
