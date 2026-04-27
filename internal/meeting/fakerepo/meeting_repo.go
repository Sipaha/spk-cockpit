// Package fakerepo provides an in-memory meeting.MeetingRepo for tests.
package fakerepo

import (
	"context"
	"sort"
	"sync"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/meeting"
)

// Meeting is an in-memory meeting.MeetingRepo.
type Meeting struct {
	mu      sync.Mutex
	byID    map[string]api.Meeting
	deleted map[string]bool
}

// NewMeeting constructs an empty in-memory meeting repo.
func NewMeeting() *Meeting {
	return &Meeting{byID: map[string]api.Meeting{}, deleted: map[string]bool{}}
}

// Create inserts m.
func (r *Meeting) Create(_ context.Context, m api.Meeting) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[m.ID] = m
	return nil
}

// Get returns a non-deleted meeting.
func (r *Meeting) Get(_ context.Context, id string) (api.Meeting, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.deleted[id] {
		return api.Meeting{}, meeting.ErrNotFound
	}
	m, ok := r.byID[id]
	if !ok {
		return api.Meeting{}, meeting.ErrNotFound
	}
	return m, nil
}

// Update applies mutate.
func (r *Meeting) Update(_ context.Context, id string, mutate func(*api.Meeting) error) (api.Meeting, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.deleted[id] {
		return api.Meeting{}, meeting.ErrNotFound
	}
	m, ok := r.byID[id]
	if !ok {
		return api.Meeting{}, meeting.ErrNotFound
	}
	if err := mutate(&m); err != nil {
		return api.Meeting{}, err
	}
	r.byID[id] = m
	return m, nil
}

// Delete soft-deletes by id.
func (r *Meeting) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[id]; !ok {
		return meeting.ErrNotFound
	}
	if r.deleted[id] {
		return meeting.ErrNotFound
	}
	r.deleted[id] = true
	return nil
}

// List filters in memory.
func (r *Meeting) List(_ context.Context, f meeting.MeetingFilter) ([]api.Meeting, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []api.Meeting
	for id, m := range r.byID {
		if r.deleted[id] {
			continue
		}
		if !f.IncludeDone && m.Cancelled {
			continue
		}
		if f.FromUnix > 0 && m.StartAt < f.FromUnix {
			continue
		}
		if f.ToUnix > 0 && m.StartAt > f.ToUnix {
			continue
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartAt < out[j].StartAt })
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

// UpsertExternal inserts-or-updates by (source, externalUID).
func (r *Meeting) UpsertExternal(_ context.Context, m api.Meeting) (api.Meeting, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, existing := range r.byID {
		if r.deleted[id] {
			continue
		}
		if existing.Source == m.Source && existing.ExternalUID == m.ExternalUID && m.ExternalUID != "" {
			preserved := existing
			preserved.ExternalETag = m.ExternalETag
			preserved.Title = m.Title
			preserved.Description = m.Description
			preserved.Location = m.Location
			preserved.StartAt = m.StartAt
			preserved.EndAt = m.EndAt
			preserved.Cancelled = m.Cancelled
			preserved.UpdatedAt = m.UpdatedAt
			if m.StartAt != existing.StartAt {
				preserved.NotifiedAt = nil
			}
			r.byID[id] = preserved
			return preserved, false, nil
		}
	}
	r.byID[m.ID] = m
	return m, true, nil
}

// MarkCancelled sets cancelled on the matching meeting.
func (r *Meeting) MarkCancelled(_ context.Context, source api.MeetingSource, externalUID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, m := range r.byID {
		if r.deleted[id] {
			continue
		}
		if m.Source == source && m.ExternalUID == externalUID {
			m.Cancelled = true
			r.byID[id] = m
		}
	}
	return nil
}

// PendingNotification scans in memory.
func (r *Meeting) PendingNotification(_ context.Context, now int64, defaultNotifyMin int) ([]api.Meeting, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []api.Meeting
	for id, m := range r.byID {
		if r.deleted[id] || m.Cancelled || m.NotifiedAt != nil {
			continue
		}
		nm := defaultNotifyMin
		if m.NotifyMin != nil {
			nm = *m.NotifyMin
		}
		if m.StartAt-int64(nm)*60 <= now && m.StartAt >= now {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartAt < out[j].StartAt })
	return out, nil
}

// MarkNotified sets notified_at.
func (r *Meeting) MarkNotified(_ context.Context, id string, at int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.byID[id]
	if !ok {
		return meeting.ErrNotFound
	}
	m.NotifiedAt = &at
	r.byID[id] = m
	return nil
}
