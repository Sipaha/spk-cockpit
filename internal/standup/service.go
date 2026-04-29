// Package standup aggregates "Yesterday / Today / Blockers" from closed todos,
// GitLab commits, and a configurable HTTP task tracker.
package standup

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/clock"
	"github.com/spk/spk-cockpit/internal/sync/gitlab"
	"github.com/spk/spk-cockpit/internal/sync/tracker"
	"github.com/spk/spk-cockpit/internal/todo"
)

// TodoQuerier is the subset of todo.Service that standup needs.
type TodoQuerier interface {
	List(ctx context.Context, f todo.TodoFilter) ([]api.Todo, error)
}

// EventLister yields all todo audit events since a given unix-second timestamp.
type EventLister interface {
	ListAll(ctx context.Context, sinceUnix int64, limit int) ([]api.TodoEvent, error)
}

// Config wires the dependencies for a Service.
type Config struct {
	Todos        TodoQuerier
	Events       EventLister
	GitLab       gitlab.Source  // nil if disabled
	Tracker      tracker.Source // nil if disabled
	GitLabAuthor string         // empty disables fan-out
	TrackerUser  string         // empty disables fan-out
	Clock        clock.Clock
}

// Service is the standup aggregator. Stateless; safe for concurrent calls.
type Service struct {
	cfg Config
}

// NewService constructs a Service.
func NewService(cfg Config) *Service { return &Service{cfg: cfg} }

// Generate builds the report for the local-day `day` (its date is used; time-of-day ignored).
//
// "Yesterday" = items active in [day-1d 00:00, day 00:00) local time.
// "Today"     = open in_progress todos + open todos with due_at within [day 00:00, day+1d) local time.
// "Blockers"  = urgent/high open todos with due_at < day 00:00 local time.
func (s *Service) Generate(ctx context.Context, day time.Time) (api.StandupReport, error) {
	loc := day.Location()
	dayStart := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc)
	dayEnd := dayStart.Add(24 * time.Hour)
	yStart := dayStart.Add(-24 * time.Hour)

	report := api.StandupReport{
		Day:       dayStart.Format("2006-01-02"),
		Yesterday: []api.StandupItem{},
		Today:     []api.StandupItem{},
		Blockers:  []api.StandupItem{},
	}

	// Each producer goroutine writes into its OWN result struct so concurrent
	// fan-out doesn't need a mutex around the report slices and the dependence
	// on the post-Wait sort is explicit at the type level — the merged output
	// is unordered until sortByAtDesc runs.
	type goroutineResult struct {
		yesterday []api.StandupItem
		today     []api.StandupItem
		blockers  []api.StandupItem
		label     string
		err       error
	}

	var (
		mu      sync.Mutex
		results []goroutineResult
		wg      sync.WaitGroup
	)
	collect := func(r goroutineResult) {
		mu.Lock()
		results = append(results, r)
		mu.Unlock()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		yest, today, blockers, err := s.todoBuckets(ctx, yStart, dayStart, dayEnd)
		collect(goroutineResult{yesterday: yest, today: today, blockers: blockers, label: "todos", err: err})
	}()

	if s.cfg.GitLab != nil && s.cfg.GitLabAuthor != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			items, err := s.gitlabBucket(ctx, yStart, dayStart)
			collect(goroutineResult{yesterday: items, label: "gitlab", err: err})
		}()
	}

	if s.cfg.Tracker != nil && s.cfg.TrackerUser != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			items, err := s.trackerBucket(ctx, yStart, dayStart)
			collect(goroutineResult{yesterday: items, label: "tracker", err: err})
		}()
	}

	wg.Wait()

	// Merge + sort once. Producer order is non-deterministic; sortByAtDesc
	// imposes the canonical newest-first order callers expect.
	var errs []string
	for _, r := range results {
		if r.err != nil {
			errs = append(errs, r.label+": "+r.err.Error())
			continue
		}
		report.Yesterday = append(report.Yesterday, r.yesterday...)
		report.Today = append(report.Today, r.today...)
		report.Blockers = append(report.Blockers, r.blockers...)
	}
	sortByAtDesc(report.Yesterday)
	sortByAtDesc(report.Today)
	sortByAtDesc(report.Blockers)

	if len(errs) > 0 {
		report.Errors = errs
		sort.Strings(report.Errors)
	}
	return report, nil
}

func (s *Service) todoBuckets(ctx context.Context, yStart, dayStart, dayEnd time.Time) (yest, today, blockers []api.StandupItem, err error) {
	if s.cfg.Events == nil || s.cfg.Todos == nil {
		return nil, nil, nil, errors.New("not configured")
	}
	allTodos, err := s.cfg.Todos.List(ctx, todo.TodoFilter{IncludeDone: true})
	if err != nil {
		return nil, nil, nil, err
	}
	titles := make(map[string]string, len(allTodos))
	for _, t := range allTodos {
		titles[t.ID] = t.Title
	}

	events, err := s.cfg.Events.ListAll(ctx, yStart.Unix(), 500)
	if err != nil {
		return nil, nil, nil, err
	}
	seen := make(map[string]struct{})
	for _, e := range events {
		if e.Kind != "status_changed" || e.ToValue != string(api.StatusDone) {
			continue
		}
		if e.At < yStart.Unix() || e.At >= dayStart.Unix() {
			continue
		}
		if _, ok := seen[e.TodoID]; ok {
			continue
		}
		seen[e.TodoID] = struct{}{}
		title, ok := titles[e.TodoID]
		if !ok {
			title = e.TodoID
		}
		yest = append(yest, api.StandupItem{
			Source: api.StandupSourceTodo,
			Title:  title,
			Detail: "done",
			RefID:  e.TodoID,
			At:     e.At,
		})
	}

	open, err := s.cfg.Todos.List(ctx, todo.TodoFilter{Statuses: []api.TodoStatus{api.StatusInProgress, api.StatusOpen}})
	if err != nil {
		return nil, nil, nil, err
	}
	for _, t := range open {
		if t.Status == api.StatusInProgress {
			today = append(today, api.StandupItem{
				Source: api.StandupSourceTodo,
				Title:  t.Title,
				Detail: "in progress",
				RefID:  t.ID,
				At:     t.UpdatedAt,
			})
			continue
		}
		if t.DueAt != nil && *t.DueAt >= dayStart.Unix() && *t.DueAt < dayEnd.Unix() {
			today = append(today, api.StandupItem{
				Source: api.StandupSourceTodo,
				Title:  t.Title,
				Detail: "due today",
				RefID:  t.ID,
				At:     *t.DueAt,
			})
			continue
		}
		if t.DueAt != nil && *t.DueAt < dayStart.Unix() && (t.Priority == api.PriorityUrgent || t.Priority == api.PriorityHigh) {
			blockers = append(blockers, api.StandupItem{
				Source: api.StandupSourceTodo,
				Title:  t.Title,
				Detail: "overdue",
				RefID:  t.ID,
				At:     *t.DueAt,
			})
		}
	}
	return yest, today, blockers, nil
}

func (s *Service) gitlabBucket(ctx context.Context, since, until time.Time) ([]api.StandupItem, error) {
	commits, err := s.cfg.GitLab.CommitsBy(ctx, s.cfg.GitLabAuthor, since, until)
	if err != nil {
		return nil, err
	}
	out := make([]api.StandupItem, 0, len(commits))
	for _, c := range commits {
		out = append(out, api.StandupItem{
			Source: api.StandupSourceGitLab,
			Title:  c.Title,
			Detail: c.Project,
			URL:    c.URL,
			RefID:  c.SHA,
			At:     c.At.Unix(),
		})
	}
	return out, nil
}

func (s *Service) trackerBucket(ctx context.Context, since, until time.Time) ([]api.StandupItem, error) {
	items, err := s.cfg.Tracker.AssignedActive(ctx, s.cfg.TrackerUser, since, until)
	if err != nil {
		return nil, err
	}
	out := make([]api.StandupItem, 0, len(items))
	for _, it := range items {
		out = append(out, api.StandupItem{
			Source: api.StandupSourceTracker,
			Title:  it.Key + ": " + it.Title,
			Detail: it.Status,
			URL:    it.URL,
			RefID:  it.ID,
			At:     it.At.Unix(),
		})
	}
	return out, nil
}

func sortByAtDesc(items []api.StandupItem) {
	sort.SliceStable(items, func(i, j int) bool { return items[i].At > items[j].At })
}
