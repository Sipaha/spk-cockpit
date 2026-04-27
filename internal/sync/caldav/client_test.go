package caldav

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/spk/spk-cockpit/internal/api"
)

func TestParseICalEvents_SingleEvent(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "caldav", "single_event.ics"))
	require.NoError(t, err)

	from := time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC)
	to := from.Add(7 * 24 * time.Hour)

	fc, err := NewFakeFromICal(data, from, to)
	require.NoError(t, err)
	require.Len(t, fc.Events, 1)
	e := fc.Events[0]
	require.Equal(t, api.MeetingSourceCalDAV, e.Source)
	require.Equal(t, "test-event-uid-1@example.com", e.ExternalUID)
	require.Equal(t, "Project sync", e.Title)
	require.Equal(t, "Meet link", e.Location)

	want := time.Date(2026, 4, 27, 14, 0, 0, 0, time.UTC).Unix()
	require.Equal(t, want, e.StartAt)
}

func TestFakeClient_FetchEvents_ReturnsConfigured(t *testing.T) {
	fc := &FakeClient{
		Events:  []api.Meeting{{Source: api.MeetingSourceCalDAV, ExternalUID: "u", Title: "T", StartAt: 100, EndAt: 200}},
		NewCTag: "ctag-1",
	}
	out, ctag, unchanged, err := fc.FetchEvents(context.Background(), time.Time{}, time.Time{}, "")
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Equal(t, "ctag-1", ctag)
	require.False(t, unchanged)
	require.Equal(t, 1, fc.Calls)
}
