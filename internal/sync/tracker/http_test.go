package tracker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHTTPSource_NotConfigured(t *testing.T) {
	_, err := NewHTTPSource(Config{BaseURL: "x"})
	require.ErrorIs(t, err, ErrNotConfigured)
}

func TestHTTPSource_AssignedActive_SkipsUnparseableTimestamps(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"records":[
			{"id":"emodel/task@TICKET-BAD","attributes":{"_disp":"Bad Date","_status":"in_progress","_modified":"not-a-date"}},
			{"id":"emodel/task@TICKET-OK","attributes":{"_disp":"Good Date","_status":"done","_modified":"2026-04-26T10:00:00Z"}}
		]}`))
	}))
	defer srv.Close()

	src, err := NewHTTPSource(Config{BaseURL: srv.URL, Username: "alice", Token: "tok"})
	require.NoError(t, err)
	since := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC)
	items, err := src.AssignedActive(context.Background(), "alice", since, until)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "TICKET-OK", items[0].Key)
}

func TestHTTPSource_AssignedActive_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/gateway/emodel/api/records/query", r.URL.Path)
		u, p, ok := r.BasicAuth()
		require.True(t, ok)
		require.Equal(t, "alice", u)
		require.Equal(t, "tok", p)

		var body queryRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, "emodel/task", body.Query.SourceID)
		require.Equal(t, "alice", body.Query.Query["assignee"])

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"records":[
			{"id":"emodel/task@TICKET-1","attributes":{"_disp":"Title 1","_status":"in_progress","_modified":"2026-04-26T12:00:00Z"}},
			{"id":"emodel/task@TICKET-2","attributes":{"_disp":"Title 2","_status":"done","_modified":"2026-04-26T18:00:00Z"}}
		]}`))
	}))
	defer srv.Close()

	src, err := NewHTTPSource(Config{BaseURL: srv.URL, Username: "alice", Token: "tok"})
	require.NoError(t, err)
	since := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC)
	items, err := src.AssignedActive(context.Background(), "alice", since, until)
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, "TICKET-2", items[0].Key)
	require.Equal(t, "done", items[0].Status)
	require.Contains(t, items[0].URL, "recordRef=emodel/task@TICKET-2")
}
