package clock

import (
	"sync"
	"time"
)

// Fake is a Clock with controllable time, suitable for deterministic tests.
type Fake struct {
	mu  sync.Mutex
	now time.Time
}

// NewFake creates a Fake clock starting at the given time.
func NewFake(t time.Time) *Fake {
	return &Fake{now: t}
}

// Now returns the Fake's current time.
func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

// Advance moves the Fake's time forward by d.
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}

// Set overrides the Fake's current time.
func (f *Fake) Set(t time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = t
}
