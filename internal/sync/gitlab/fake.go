package gitlab

import (
	"context"
	"sort"
	"sync"
	"time"
)

// Fake is an in-memory Source for tests. Safe for concurrent use.
type Fake struct {
	mu      sync.Mutex
	commits []Commit
	err     error
}

// NewFake returns an empty Fake.
func NewFake() *Fake { return &Fake{} }

// SetCommits replaces the canned commit list.
func (f *Fake) SetCommits(cs []Commit) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commits = append([]Commit(nil), cs...)
}

// SetError sets a sticky error returned by every CommitsBy call.
func (f *Fake) SetError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

// CommitsBy filters the canned list by author and range.
func (f *Fake) CommitsBy(_ context.Context, author string, since, until time.Time) ([]Commit, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	out := make([]Commit, 0, len(f.commits))
	_ = author // fake ignores author filter; tests pre-populate matching commits
	for _, c := range f.commits {
		if (c.At.Equal(since) || c.At.After(since)) && c.At.Before(until) {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At.After(out[j].At) })
	return out, nil
}
