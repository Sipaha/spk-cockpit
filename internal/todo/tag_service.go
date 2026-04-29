package todo

import (
	"context"
	"errors"
	"strings"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/clock"
)

// ErrTagNameRequired is returned when an empty tag name is submitted.
var ErrTagNameRequired = errors.New("tag: name required")

// TagService wraps TagRepo with a domain-aware entry point so HTTP and CLI
// callers don't bypass the service layer to mutate tags. Centralising the
// stamping (CreatedAt) and the validation here keeps audit / event hooks a
// one-line change later — adding them in the handler would have to touch
// every route.
type TagService struct {
	repo  TagRepo
	clock clock.Clock
	bus   api.EventPublisher
}

// NewTagService wires the service. bus may be nil — methods are nil-safe.
func NewTagService(r TagRepo, c clock.Clock, bus api.EventPublisher) *TagService {
	return &TagService{repo: r, clock: c, bus: bus}
}

// Repo returns the underlying repo for read-only callers (e.g. the HTTP
// handler that maps todos → tags during list rendering). Callers that
// mutate must go through the TagService methods to keep the invariants.
func (s *TagService) Repo() TagRepo { return s.repo }

// List returns all known tags.
func (s *TagService) List(ctx context.Context) ([]api.Tag, error) {
	return s.repo.List(ctx)
}

// Upsert creates or updates a tag, stamping CreatedAt from the injected clock.
func (s *TagService) Upsert(ctx context.Context, name, color string) (api.Tag, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return api.Tag{}, ErrTagNameRequired
	}
	t := api.Tag{Name: name, Color: color, CreatedAt: s.clock.Now().Unix()}
	if err := s.repo.Upsert(ctx, t); err != nil {
		return api.Tag{}, err
	}
	return t, nil
}

// Rename renames a tag. SQLite's `ON UPDATE CASCADE` keeps todo→tag links in
// sync, so we don't need a separate sweep.
func (s *TagService) Rename(ctx context.Context, oldName, newName string) error {
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if oldName == "" || newName == "" {
		return ErrTagNameRequired
	}
	return s.repo.Rename(ctx, oldName, newName)
}

// Delete removes a tag.
func (s *TagService) Delete(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrTagNameRequired
	}
	return s.repo.Delete(ctx, name)
}
