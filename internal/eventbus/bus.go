// Package eventbus provides a tiny in-process pub/sub used by domain services and SSE handlers.
package eventbus

import (
	"sync"

	"github.com/spk/spk-cockpit/internal/api"
)

// Bus is an in-memory pub/sub. Slow subscribers drop events (Publish never blocks).
type Bus struct {
	mu     sync.RWMutex
	subs   map[chan api.Event]struct{}
	closed bool
}

// New creates a Bus. The argument is reserved for future tuning and currently ignored.
func New(_ int) *Bus {
	return &Bus{subs: map[chan api.Event]struct{}{}}
}

// Subscribe creates a new subscriber channel with the given buffer.
func (b *Bus) Subscribe(buf int) chan api.Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan api.Event, buf)
	if b.closed {
		close(ch)
		return ch
	}
	b.subs[ch] = struct{}{}
	return ch
}

// Unsubscribe removes a subscriber channel and closes it. Idempotent.
func (b *Bus) Unsubscribe(ch chan api.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.subs[ch]; !ok {
		return
	}
	delete(b.subs, ch)
	close(ch)
}

// Publish best-effort sends e to all subscribers, dropping for full channels.
func (b *Bus) Publish(e api.Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return
	}
	for ch := range b.subs {
		select {
		case ch <- e:
		default:
		}
	}
}

// Close stops accepting publishes and closes all current subscriber channels.
func (b *Bus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for ch := range b.subs {
		close(ch)
	}
	b.subs = nil
}
