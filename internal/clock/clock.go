// Package clock provides a Clock abstraction (real and fake) for tests.
package clock

import "time"

// Clock returns the current time.
type Clock interface {
	Now() time.Time
}

type realClock struct{}

// Real returns a Clock backed by time.Now().UTC().
func Real() Clock { return realClock{} }

func (realClock) Now() time.Time { return time.Now().UTC() }
