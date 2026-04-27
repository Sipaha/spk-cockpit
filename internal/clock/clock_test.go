package clock

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRealClock_NowReturnsCurrentTime(t *testing.T) {
	c := Real()
	before := time.Now()
	got := c.Now()
	after := time.Now()
	require.True(t, !got.Before(before) && !got.After(after))
}

func TestFakeClock_NowReturnsSetValue(t *testing.T) {
	t0 := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	c := NewFake(t0)
	require.Equal(t, t0, c.Now())
}

func TestFakeClock_AdvanceMovesNow(t *testing.T) {
	t0 := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	c := NewFake(t0)
	c.Advance(5 * time.Minute)
	require.Equal(t, t0.Add(5*time.Minute), c.Now())
}
