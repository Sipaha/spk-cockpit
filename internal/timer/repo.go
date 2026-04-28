// Package timer holds the time-tracking domain (service, repository contract, errors).
package timer

import (
	"context"
	"errors"

	"github.com/spk/spk-cockpit/internal/api"
)

// Domain errors.
var (
	// ErrSessionNotFound is returned when a session id does not exist.
	ErrSessionNotFound = errors.New("timer: session not found")
	// ErrNoActiveSession is returned when Stop is called and nothing is running.
	ErrNoActiveSession = errors.New("timer: no active session")
	// ErrAlreadyActiveOnTodo is returned by repo.Start when the partial unique index trips.
	// Service-level Start handles the "one active globally" rule by stopping any current
	// session first, so this error from the repo indicates a bug.
	ErrAlreadyActiveOnTodo = errors.New("timer: another active session exists on this todo")
)

// TimerRepo persists time-tracking sessions.
//
//nolint:revive // domain naming intentional
type TimerRepo interface {
	// Start inserts a new session with EndedAt nil and returns its id. If the
	// partial unique index trips, ErrAlreadyActiveOnTodo is returned.
	Start(ctx context.Context, todoID string, startedAt int64, source string) (int64, error)

	// Stop sets EndedAt on the active session of todoID. Returns the updated row.
	// If no active session exists, ErrNoActiveSession.
	Stop(ctx context.Context, todoID string, endedAt int64) (api.TimerSession, error)

	// Active returns one currently-active session if any exists. Kept for
	// callers that just need to know "is something running"; with parallel
	// timers, ListActive is the more honest API.
	Active(ctx context.Context) (*api.TimerSession, error)

	// ListActive returns every currently-running session. Order is undefined.
	ListActive(ctx context.Context) ([]api.TimerSession, error)

	// ActiveByTodo returns the active session belonging to todoID, or
	// (nil, nil) if that todo has no running timer.
	ActiveByTodo(ctx context.Context, todoID string) (*api.TimerSession, error)

	// ListByTodo returns all sessions for a todo, newest first.
	ListByTodo(ctx context.Context, todoID string, limit int) ([]api.TimerSession, error)

	// TotalForTodo returns aggregated seconds for completed sessions of todoID
	// with started_at >= sinceUnix. Active (unfinished) session is NOT included.
	TotalForTodo(ctx context.Context, todoID string, sinceUnix int64) (int64, int, error)
}
