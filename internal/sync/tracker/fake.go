package tracker

import (
	"context"
	"sort"
	"sync"
	"time"
)

// Fake is an in-memory Source for tests.
type Fake struct {
	mu    sync.Mutex
	items []Item
	err   error
}

// NewFake returns an empty Fake.
func NewFake() *Fake { return &Fake{} }

// SetItems replaces the canned item list.
func (f *Fake) SetItems(it []Item) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.items = append([]Item(nil), it...)
}

// SetError sets a sticky error returned by every AssignedActive call.
func (f *Fake) SetError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

// AssignedActive filters the canned list by range.
func (f *Fake) AssignedActive(_ context.Context, username string, since, until time.Time) ([]Item, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	_ = username
	out := make([]Item, 0, len(f.items))
	for _, it := range f.items {
		if (it.At.Equal(since) || it.At.After(since)) && it.At.Before(until) {
			out = append(out, it)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At.After(out[j].At) })
	return out, nil
}
