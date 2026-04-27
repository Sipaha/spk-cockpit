package meeting_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/clock"
	"github.com/spk/spk-cockpit/internal/meeting"
	"github.com/spk/spk-cockpit/internal/meeting/fakerepo"
)

func newSvc(t *testing.T, t0 time.Time) (*meeting.Service, *fakerepo.Meeting) {
	t.Helper()
	r := fakerepo.NewMeeting()
	c := clock.NewFake(t0)
	return meeting.NewService(r, c, nil), r
}

func TestService_CreateManual_AssignsIDAndTimestamps(t *testing.T) {
	t0 := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	s, _ := newSvc(t, t0)
	got, err := s.CreateManual(context.Background(), api.CreateMeetingRequest{
		Title: "Standup", StartAt: t0.Add(time.Hour).Unix(), EndAt: t0.Add(90 * time.Minute).Unix(),
	})
	require.NoError(t, err)
	require.NotEmpty(t, got.ID)
	require.Equal(t, api.MeetingSourceManual, got.Source)
	require.Equal(t, "Standup", got.Title)
	require.Equal(t, t0.Unix(), got.CreatedAt)
}

func TestService_CreateManual_RejectsBadRange(t *testing.T) {
	s, _ := newSvc(t, time.Now())
	_, err := s.CreateManual(context.Background(), api.CreateMeetingRequest{
		Title: "Bad", StartAt: 200, EndAt: 100,
	})
	require.ErrorIs(t, err, meeting.ErrInvalidRange)
}

func TestService_UpdateManual_OnlyManual(t *testing.T) {
	t0 := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	s, r := newSvc(t, t0)
	cal := api.Meeting{
		ID: "ext", Source: api.MeetingSourceCalDAV, ExternalUID: "uid", Title: "From cal",
		StartAt: t0.Unix(), EndAt: t0.Add(time.Hour).Unix(),
		CreatedAt: t0.Unix(), UpdatedAt: t0.Unix(),
	}
	require.NoError(t, r.Create(context.Background(), cal))

	newTitle := "edited"
	_, err := s.UpdateManual(context.Background(), "ext", api.UpdateMeetingRequest{Title: &newTitle})
	require.ErrorIs(t, err, meeting.ErrManualOnly)
}

func TestService_UpsertFromCalDAV_ReschedulesResetsNotification(t *testing.T) {
	t0 := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	s, r := newSvc(t, t0)
	ctx := context.Background()

	first, err := s.UpsertFromCalDAV(ctx, api.Meeting{
		ID: "x", Source: api.MeetingSourceCalDAV, ExternalUID: "uid-1", ExternalETag: "e1",
		Title: "Sync me", StartAt: t0.Add(time.Hour).Unix(), EndAt: t0.Add(2 * time.Hour).Unix(),
	})
	require.NoError(t, err)
	require.Equal(t, "Sync me", first.Title)

	require.NoError(t, r.MarkNotified(ctx, first.ID, t0.Unix()))

	second, err := s.UpsertFromCalDAV(ctx, api.Meeting{
		Source: api.MeetingSourceCalDAV, ExternalUID: "uid-1", ExternalETag: "e2",
		Title: "Sync me — renamed", StartAt: first.StartAt, EndAt: first.EndAt,
	})
	require.NoError(t, err)
	require.Equal(t, "Sync me — renamed", second.Title)
	require.NotNil(t, second.NotifiedAt)

	third, err := s.UpsertFromCalDAV(ctx, api.Meeting{
		Source: api.MeetingSourceCalDAV, ExternalUID: "uid-1", ExternalETag: "e3",
		Title: "Sync me — renamed", StartAt: first.StartAt + 600, EndAt: first.EndAt + 600,
	})
	require.NoError(t, err)
	require.Nil(t, third.NotifiedAt)
}

func TestService_NextMeeting_ReturnsEarliestUpcoming(t *testing.T) {
	t0 := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	s, _ := newSvc(t, t0)
	ctx := context.Background()
	_, _ = s.CreateManual(ctx, api.CreateMeetingRequest{
		Title: "Later", StartAt: t0.Add(2 * time.Hour).Unix(), EndAt: t0.Add(3 * time.Hour).Unix(),
	})
	_, _ = s.CreateManual(ctx, api.CreateMeetingRequest{
		Title: "Sooner", StartAt: t0.Add(time.Hour).Unix(), EndAt: t0.Add(90 * time.Minute).Unix(),
	})
	_, _ = s.CreateManual(ctx, api.CreateMeetingRequest{
		Title: "Past", StartAt: t0.Add(-time.Hour).Unix(), EndAt: t0.Add(-30 * time.Minute).Unix(),
	})

	next, err := s.Next(ctx)
	require.NoError(t, err)
	require.NotNil(t, next)
	require.Equal(t, "Sooner", next.Title)
}

func TestService_Next_NilWhenEmpty(t *testing.T) {
	s, _ := newSvc(t, time.Now())
	got, err := s.Next(context.Background())
	require.NoError(t, err)
	require.Nil(t, got)
}
