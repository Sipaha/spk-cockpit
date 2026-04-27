// Package tracker is a read-only client for Citeck Project Tracker, used by the
// standup aggregator. No write APIs are exposed.
package tracker

import (
	"context"
	"errors"
	"time"
)

// ErrNotConfigured is returned when Tracker is not configured (missing URL/token/username).
var ErrNotConfigured = errors.New("tracker: not configured")

// Item is a minimal tracker record for standup display.
type Item struct {
	ID     string // record ref e.g. "task@TICKET-123"
	Key    string // display key e.g. "TICKET-123"
	Title  string
	Status string    // current status
	URL    string    // https://tracker/.../v_app/task@TICKET-123
	At     time.Time // last-modified
}

// Source fetches tracker items recently active for the configured user.
type Source interface {
	// AssignedActive returns items assigned to `username` whose modifiedAt falls in
	// [since, until). Sorted DESC by At.
	AssignedActive(ctx context.Context, username string, since, until time.Time) ([]Item, error)
}
