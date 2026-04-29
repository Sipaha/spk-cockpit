package caldav_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/clock"
	"github.com/spk/spk-cockpit/internal/meeting"
	meetingfake "github.com/spk/spk-cockpit/internal/meeting/fakerepo"
	"github.com/spk/spk-cockpit/internal/sync/caldav"
)

type stateRepoFake struct {
	entry api.SyncStateEntry
}

func (f *stateRepoFake) Get(_ context.Context, source string) (api.SyncStateEntry, error) {
	if f.entry.Source == source {
		return f.entry, nil
	}
	return api.SyncStateEntry{Source: source}, nil
}
func (f *stateRepoFake) Save(_ context.Context, e api.SyncStateEntry) error { f.entry = e; return nil }

func TestSyncer_RunOnce_UpsertsEventsAndMarksMissingCancelled(t *testing.T) {
	t0 := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	mrepo := meetingfake.NewMeeting()
	c := clock.NewFake(t0)
	msvc := meeting.NewService(mrepo, c, nil)

	require.NoError(t, mrepo.Create(context.Background(), api.Meeting{
		ID: "old", Source: api.MeetingSourceCalDAV, ExternalUID: "old-uid",
		Title: "Old", StartAt: t0.Add(time.Hour).Unix(), EndAt: t0.Add(2 * time.Hour).Unix(),
		CreatedAt: t0.Unix(), UpdatedAt: t0.Unix(),
	}))

	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "caldav", "single_event.ics"))
	require.NoError(t, err)
	fc, err := caldav.NewFakeFromICal(data, t0.Add(-7*24*time.Hour), t0.Add(30*24*time.Hour))
	require.NoError(t, err)

	state := &stateRepoFake{}
	s := caldav.NewSyncer(caldav.SyncerConfig{
		Client: fc, Meetings: msvc, State: state, Clock: c, Interval: time.Hour,
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.Run(ctx)

	got, err := mrepo.Get(context.Background(), "old")
	require.NoError(t, err)
	require.True(t, got.Cancelled)

	listed, err := msvc.List(context.Background(), meeting.MeetingFilter{FromUnix: t0.Add(-time.Hour).Unix(), ToUnix: t0.Add(7 * 24 * time.Hour).Unix()})
	require.NoError(t, err)
	var found bool
	for _, m := range listed {
		if m.ExternalUID == "test-event-uid-1@example.com" {
			found = true
		}
	}
	require.True(t, found)
}

func TestSyncer_RunOnce_PropagatesErrorToState(t *testing.T) {
	t0 := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	mrepo := meetingfake.NewMeeting()
	c := clock.NewFake(t0)
	msvc := meeting.NewService(mrepo, c, nil)

	fc := &caldav.FakeClient{Err: errStub("boom")}
	state := &stateRepoFake{}
	s := caldav.NewSyncer(caldav.SyncerConfig{
		Client: fc, Meetings: msvc, State: state, Clock: c, Interval: time.Hour,
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.Run(ctx)

	require.Equal(t, "boom", state.entry.LastErr)
	require.Nil(t, state.entry.LastOkAt)
	st := s.Status()
	require.Equal(t, "boom", st[0].LastErr)
}

type errStub string

func (e errStub) Error() string { return string(e) }
