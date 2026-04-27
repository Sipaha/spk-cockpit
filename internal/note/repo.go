// Package note holds short-form attached notes (markdown body, attached to meeting or todo).
package note

import (
	"context"
	"errors"

	"github.com/spk/spk-cockpit/internal/api"
)

// ErrNotFound is returned when a note id does not exist.
var ErrNotFound = errors.New("note: not found")

// NoteFilter narrows List. //nolint:revive // intentional domain naming
type NoteFilter struct { //nolint:revive // intentional domain naming
	MeetingID string
	TodoID    string
	Limit     int
}

// NoteRepo persists notes. //nolint:revive // domain naming intentional
type NoteRepo interface { //nolint:revive // domain naming intentional
	Upsert(ctx context.Context, n api.Note) error
	Get(ctx context.Context, id string) (api.Note, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, f NoteFilter) ([]api.Note, error)
	FindByAttachment(ctx context.Context, meetingID, todoID string) (api.Note, error)
}
