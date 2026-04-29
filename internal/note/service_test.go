package note_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/clock"
	"github.com/spk/spk-cockpit/internal/note"
	"github.com/spk/spk-cockpit/internal/note/fakerepo"
)

func TestService_Upsert_AttachToMeeting_AssignsIDAndTimestamps(t *testing.T) {
	t0 := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	s := note.NewService(fakerepo.NewNote(), clock.NewFake(t0))

	got, err := s.Upsert(context.Background(), api.UpsertNoteRequest{MeetingID: "m-1", Body: "hello"})
	require.NoError(t, err)
	require.NotEmpty(t, got.ID)
	require.Equal(t, "m-1", got.MeetingID)
	require.Equal(t, "hello", got.Body)
	require.Equal(t, t0.Unix(), got.CreatedAt)
	require.Equal(t, t0.Unix(), got.UpdatedAt)
}

func TestService_Upsert_ReusesExistingNoteOnSameMeeting(t *testing.T) {
	t0 := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	c := clock.NewFake(t0)
	s := note.NewService(fakerepo.NewNote(), c)
	ctx := context.Background()

	first, err := s.Upsert(ctx, api.UpsertNoteRequest{MeetingID: "m-1", Body: "v1"})
	require.NoError(t, err)

	c.Advance(time.Minute)
	second, err := s.Upsert(ctx, api.UpsertNoteRequest{MeetingID: "m-1", Body: "v2"})
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID, "same meeting should reuse same note id")
	require.Equal(t, "v2", second.Body)
	require.Equal(t, first.CreatedAt, second.CreatedAt)
	require.Greater(t, second.UpdatedAt, first.UpdatedAt)
}

func TestService_Upsert_RequiresExactlyOneAttachment(t *testing.T) {
	s := note.NewService(fakerepo.NewNote(), clock.NewFake(time.Now()))

	_, err := s.Upsert(context.Background(), api.UpsertNoteRequest{Body: "no attachment"})
	require.Error(t, err)

	_, err = s.Upsert(context.Background(), api.UpsertNoteRequest{MeetingID: "m", TodoID: "t", Body: "both"})
	require.Error(t, err)
}

func TestService_Delete(t *testing.T) {
	s := note.NewService(fakerepo.NewNote(), clock.NewFake(time.Now()))
	got, _ := s.Upsert(context.Background(), api.UpsertNoteRequest{MeetingID: "m-1", Body: "x"})
	require.NoError(t, s.Delete(context.Background(), got.ID))
	_, err := s.Get(context.Background(), got.ID)
	require.ErrorIs(t, err, note.ErrNotFound)
}

func TestService_FindByMeeting_NilWhenAbsent(t *testing.T) {
	s := note.NewService(fakerepo.NewNote(), clock.NewFake(time.Now()))
	got, err := s.FindByMeeting(context.Background(), "m-x")
	require.NoError(t, err)
	require.Nil(t, got)
}
