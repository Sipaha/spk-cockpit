package secret_test

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/spk/spk-cockpit/internal/clock"
	"github.com/spk/spk-cockpit/internal/secret"
	"github.com/spk/spk-cockpit/internal/secret/fakerepo"
)

func newSvc(t *testing.T) *secret.Service {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	s, err := secret.NewService(fakerepo.NewSecret(), clock.NewFake(time.Unix(1700000000, 0)), key)
	require.NoError(t, err)
	return s
}

func TestService_SetAndGet_Roundtrip(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()

	require.NoError(t, s.Set(ctx, "yandex_caldav", "secret-password-123"))
	got, err := s.Get(ctx, "yandex_caldav")
	require.NoError(t, err)
	require.Equal(t, "secret-password-123", got)
}

func TestService_Get_NotFound(t *testing.T) {
	s := newSvc(t)
	_, err := s.Get(context.Background(), "missing")
	require.ErrorIs(t, err, secret.ErrNotFound)
}

func TestService_DifferentMasterKey_FailsToDecrypt(t *testing.T) {
	repo := fakerepo.NewSecret()
	c := clock.NewFake(time.Unix(1700000000, 0))

	key1 := make([]byte, 32)
	_, err := rand.Read(key1)
	require.NoError(t, err)
	s1, err := secret.NewService(repo, c, key1)
	require.NoError(t, err)
	require.NoError(t, s1.Set(context.Background(), "x", "v"))

	key2 := make([]byte, 32)
	_, err = rand.Read(key2)
	require.NoError(t, err)
	s2, err := secret.NewService(repo, c, key2)
	require.NoError(t, err)
	_, err = s2.Get(context.Background(), "x")
	require.Error(t, err)
}

func TestService_NewService_RequiresThirtyTwoByteKey(t *testing.T) {
	_, err := secret.NewService(fakerepo.NewSecret(), clock.NewFake(time.Now()), make([]byte, 16))
	require.Error(t, err)
}
