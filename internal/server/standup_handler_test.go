package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/clock"
	"github.com/spk/spk-cockpit/internal/eventbus"
	"github.com/spk/spk-cockpit/internal/standup"
	"github.com/spk/spk-cockpit/internal/todo"
	"github.com/spk/spk-cockpit/internal/todo/fakerepo"
)

type fakeEvents struct{}

func (fakeEvents) ListAll(_ context.Context, _ int64, _ int) ([]api.TodoEvent, error) {
	return nil, nil
}
func (fakeEvents) Append(_ context.Context, _ api.TodoEvent) error { return nil }
func (fakeEvents) ListByTodo(_ context.Context, _ string, _ int) ([]api.TodoEvent, error) {
	return nil, nil
}

func TestStandupHandler_Empty(t *testing.T) {
	bus := eventbus.New(4)
	defer bus.Close()
	svc := todo.NewService(fakerepo.NewTodo(), fakerepo.NewTag(), fakeEvents{}, clock.Real(), bus)
	st := standup.NewService(standup.Config{Todos: svc, Events: fakeEvents{}, Clock: clock.Real()})

	deps := &Deps{Standup: st}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/standup", handleStandup(deps))

	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/standup?date=2026-04-27")
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var got api.StandupReport
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	require.Equal(t, "2026-04-27", got.Day)
	require.NotNil(t, got.Yesterday)
}

func TestStandupHandler_BadDate(t *testing.T) {
	deps := &Deps{Standup: standup.NewService(standup.Config{Clock: clock.Real()})}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/standup", handleStandup(deps))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/standup?date=oops")
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestStandupHandler_Unconfigured(t *testing.T) {
	deps := &Deps{} // Standup is nil
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/standup", handleStandup(deps))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/standup")
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}
