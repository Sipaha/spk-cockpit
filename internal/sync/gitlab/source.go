// Package gitlab is a read-only client for fetching the current user's recent
// commits, used by the standup aggregator. No write APIs are exposed.
package gitlab

import (
	"context"
	"errors"
	"time"
)

// ErrNotConfigured is returned when GitLab is not configured (missing URL/token/author).
var ErrNotConfigured = errors.New("gitlab: not configured")

// Commit is a minimal commit record for standup display.
type Commit struct {
	SHA     string    // commit SHA (full or abbreviated, as returned by the API)
	Title   string    // commit message subject (first line)
	URL     string    // https://gitlab.example.com/group/proj/-/commit/<sha>
	Project string    // "group/project"
	At      time.Time // commit timestamp (UTC)
}

// Source fetches commits authored by a configured user in a time window.
type Source interface {
	// CommitsBy returns commits authored by `author` between `since` (inclusive) and
	// `until` (exclusive). Returned commits are sorted DESC by At.
	CommitsBy(ctx context.Context, author string, since, until time.Time) ([]Commit, error)
}
