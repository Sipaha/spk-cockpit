package meeting

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"

	"github.com/oklog/ulid/v2"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/clock"
)

// EventPublisher publishes domain events. May be nil — service is nil-safe.
type EventPublisher interface {
	Publish(api.Event)
}

// Service is the meeting domain entry point.
type Service struct {
	repo  MeetingRepo
	clock clock.Clock
	bus   EventPublisher
}

// NewService wires the service.
func NewService(r MeetingRepo, c clock.Clock, bus EventPublisher) *Service {
	return &Service{repo: r, clock: c, bus: bus}
}

// Clock exposes the injected clock (used by tests / scheduler).
func (s *Service) Clock() clock.Clock { return s.clock }

func (s *Service) publish(t string, data any) {
	if s.bus == nil {
		return
	}
	s.bus.Publish(api.Event{Type: t, Data: data})
}

// CreateManual creates a manual meeting and returns it.
func (s *Service) CreateManual(ctx context.Context, req api.CreateMeetingRequest) (api.Meeting, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return api.Meeting{}, errors.New("title is required")
	}
	if req.EndAt <= req.StartAt {
		return api.Meeting{}, ErrInvalidRange
	}
	now := s.clock.Now().Unix()
	m := api.Meeting{
		ID:          ulid.MustNew(ulid.Now(), rand.Reader).String(),
		Source:      api.MeetingSourceManual,
		Title:       title,
		Description: req.Description,
		Location:    req.Location,
		StartAt:     req.StartAt,
		EndAt:       req.EndAt,
		NotifyMin:   req.NotifyMin,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.repo.Create(ctx, m); err != nil {
		return api.Meeting{}, fmt.Errorf("create: %w", err)
	}
	s.publish(api.EventMeetingUpserted, api.MeetingUpsertedData{Meeting: m})
	return m, nil
}

// UpdateManual mutates a manual-source meeting. CalDAV meetings are read-only.
func (s *Service) UpdateManual(ctx context.Context, id string, req api.UpdateMeetingRequest) (api.Meeting, error) {
	now := s.clock.Now().Unix()
	updated, err := s.repo.Update(ctx, id, func(m *api.Meeting) error {
		if m.Source != api.MeetingSourceManual {
			return ErrManualOnly
		}
		if req.Title != nil {
			t := strings.TrimSpace(*req.Title)
			if t == "" {
				return errors.New("title is required")
			}
			m.Title = t
		}
		if req.Description != nil {
			m.Description = *req.Description
		}
		if req.Location != nil {
			m.Location = *req.Location
		}
		if req.StartAt != nil && *req.StartAt != m.StartAt {
			m.StartAt = *req.StartAt
			m.NotifiedAt = nil
		}
		if req.EndAt != nil {
			m.EndAt = *req.EndAt
		}
		if m.EndAt <= m.StartAt {
			return ErrInvalidRange
		}
		if req.NotifyMin != nil {
			m.NotifyMin = req.NotifyMin
			m.NotifiedAt = nil
		}
		m.UpdatedAt = now
		return nil
	})
	if err != nil {
		return api.Meeting{}, err
	}
	s.publish(api.EventMeetingUpserted, api.MeetingUpsertedData{Meeting: updated})
	return updated, nil
}

// DeleteManual soft-deletes a manual meeting.
func (s *Service) DeleteManual(ctx context.Context, id string) error {
	cur, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if cur.Source != api.MeetingSourceManual {
		return ErrManualOnly
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.publish(api.EventMeetingDeleted, api.MeetingDeletedData{MeetingID: id})
	return nil
}

// Get loads a meeting (any source).
func (s *Service) Get(ctx context.Context, id string) (api.Meeting, error) {
	return s.repo.Get(ctx, id)
}

// List returns meetings in the given range.
func (s *Service) List(ctx context.Context, f MeetingFilter) ([]api.Meeting, error) {
	return s.repo.List(ctx, f)
}

// Next returns the earliest upcoming non-cancelled meeting, or nil.
func (s *Service) Next(ctx context.Context) (*api.Meeting, error) {
	now := s.clock.Now().Unix()
	list, err := s.repo.List(ctx, MeetingFilter{FromUnix: now, Limit: 1})
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	m := list[0]
	return &m, nil
}

// UpsertFromCalDAV inserts or updates a CalDAV-sourced meeting.
func (s *Service) UpsertFromCalDAV(ctx context.Context, m api.Meeting) (api.Meeting, error) {
	if m.Source != api.MeetingSourceCalDAV || m.ExternalUID == "" {
		return api.Meeting{}, errors.New("UpsertFromCalDAV: source must be 'caldav' and ExternalUID required")
	}
	if m.EndAt <= m.StartAt {
		return api.Meeting{}, ErrInvalidRange
	}
	now := s.clock.Now().Unix()
	if m.ID == "" {
		m.ID = ulid.MustNew(ulid.Now(), rand.Reader).String()
	}
	if m.CreatedAt == 0 {
		m.CreatedAt = now
	}
	m.UpdatedAt = now

	out, _, err := s.repo.UpsertExternal(ctx, m)
	if err != nil {
		return api.Meeting{}, err
	}
	s.publish(api.EventMeetingUpserted, api.MeetingUpsertedData{Meeting: out})
	return out, nil
}

// MarkExternalCancelled flags a CalDAV meeting as cancelled.
func (s *Service) MarkExternalCancelled(ctx context.Context, externalUID string) error {
	return s.repo.MarkCancelled(ctx, api.MeetingSourceCalDAV, externalUID)
}

// PendingNotification proxies to the repo (used by the scheduler).
func (s *Service) PendingNotification(ctx context.Context, defaultNotifyMin int) ([]api.Meeting, error) {
	return s.repo.PendingNotification(ctx, s.clock.Now().Unix(), defaultNotifyMin)
}

// MarkNotified records that a meeting was successfully notified.
func (s *Service) MarkNotified(ctx context.Context, id string) error {
	now := s.clock.Now().Unix()
	if err := s.repo.MarkNotified(ctx, id, now); err != nil {
		return err
	}
	s.publish(api.EventMeetingNotificationFired, api.MeetingNotificationFiredData{
		MeetingID: id, FiredAt: now,
	})
	return nil
}
