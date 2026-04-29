package note

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/oklog/ulid/v2"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/clock"
)

// Service is the note domain entry point.
type Service struct {
	repo  NoteRepo
	clock clock.Clock
}

// NewService wires the service.
func NewService(r NoteRepo, c clock.Clock) *Service {
	return &Service{repo: r, clock: c}
}

// Upsert creates or updates the (single) note attached to a meeting OR a todo.
// Exactly one of MeetingID/TodoID must be non-empty.
func (s *Service) Upsert(ctx context.Context, req api.UpsertNoteRequest) (api.Note, error) {
	if req.MeetingID == "" && req.TodoID == "" {
		return api.Note{}, errors.New("MeetingID or TodoID is required")
	}
	if req.MeetingID != "" && req.TodoID != "" {
		return api.Note{}, errors.New("only one of MeetingID / TodoID may be set")
	}
	now := s.clock.Now().Unix()

	existing, err := s.repo.FindByAttachment(ctx, req.MeetingID, req.TodoID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return api.Note{}, fmt.Errorf("find: %w", err)
	}
	var n api.Note
	if errors.Is(err, ErrNotFound) {
		n = api.Note{
			ID:        ulid.MustNew(ulid.Now(), rand.Reader).String(),
			MeetingID: req.MeetingID,
			TodoID:    req.TodoID,
			Body:      req.Body,
			CreatedAt: now,
			UpdatedAt: now,
		}
	} else {
		n = existing
		n.Body = req.Body
		n.UpdatedAt = now
	}
	if err := s.repo.Upsert(ctx, n); err != nil {
		return api.Note{}, fmt.Errorf("upsert: %w", err)
	}
	return n, nil
}

// Get loads a note by id.
func (s *Service) Get(ctx context.Context, id string) (api.Note, error) {
	return s.repo.Get(ctx, id)
}

// Delete soft-deletes a note.
func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// FindByMeeting returns the note attached to a meeting, or (nil, nil) if absent.
func (s *Service) FindByMeeting(ctx context.Context, meetingID string) (*api.Note, error) {
	n, err := s.repo.FindByAttachment(ctx, meetingID, "")
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &n, nil
}

// FindByTodo returns the note attached to a todo, or (nil, nil) if absent.
func (s *Service) FindByTodo(ctx context.Context, todoID string) (*api.Note, error) {
	n, err := s.repo.FindByAttachment(ctx, "", todoID)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &n, nil
}
