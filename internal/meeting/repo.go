// Package meeting holds the meeting domain (service, repository contract, errors).
package meeting

import (
	"context"
	"errors"

	"github.com/spk/spk-cockpit/internal/api"
)

// Domain errors.
var (
	ErrNotFound     = errors.New("meeting: not found")
	ErrManualOnly   = errors.New("meeting: only manual meetings may be edited")
	ErrInvalidRange = errors.New("meeting: end_at must be > start_at")
)

// MeetingFilter narrows MeetingRepo.List. //nolint:revive // intentional domain naming
type MeetingFilter struct { //nolint:revive // intentional domain naming
	FromUnix    int64
	ToUnix      int64
	IncludeDone bool
	Limit       int
}

// MeetingRepo persists meetings. //nolint:revive // domain naming intentional
type MeetingRepo interface { //nolint:revive // domain naming intentional
	Create(ctx context.Context, m api.Meeting) error
	Get(ctx context.Context, id string) (api.Meeting, error)
	Update(ctx context.Context, id string, mutate func(*api.Meeting) error) (api.Meeting, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, f MeetingFilter) ([]api.Meeting, error)

	UpsertExternal(ctx context.Context, m api.Meeting) (api.Meeting, bool, error)
	MarkCancelled(ctx context.Context, source api.MeetingSource, externalUID string) error

	PendingNotification(ctx context.Context, now int64, defaultNotifyMin int) ([]api.Meeting, error)
	MarkNotified(ctx context.Context, id string, at int64) error

	PendingPopup(ctx context.Context, now int64, defaultPopupMin int) ([]api.Meeting, error)
	MarkPopupFired(ctx context.Context, id string, at int64) error
}

// SyncStateRepo tracks per-source sync cursors and last-error strings.
type SyncStateRepo interface {
	Get(ctx context.Context, source string) (api.SyncStateEntry, error)
	Save(ctx context.Context, entry api.SyncStateEntry) error
	List(ctx context.Context) ([]api.SyncStateEntry, error)
}
