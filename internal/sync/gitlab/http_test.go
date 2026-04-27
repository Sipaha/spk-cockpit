package gitlab

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHTTPSource_NotConfigured(t *testing.T) {
	_, err := NewHTTPSource(Config{})
	require.ErrorIs(t, err, ErrNotConfigured)
}

func TestHTTPSource_CommitsBy_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "tok", r.Header.Get("PRIVATE-TOKEN"))
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v4/users") && r.URL.Query().Get("username") == "alice":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id": 42}]`))
		case strings.HasPrefix(r.URL.Path, "/api/v4/users/42/events"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[
				{"created_at":"2026-04-26T10:30:00Z","action_name":"pushed to","project_id":7,
				 "push_data":{"commit_to":"abc1234567def","commit_title":"feat: thing","ref":"main"}}
			]`))
		case strings.HasPrefix(r.URL.Path, "/api/v4/projects/7"):
			_, _ = w.Write([]byte(`{"path_with_namespace":"team/project"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	src, err := NewHTTPSource(Config{BaseURL: srv.URL, Token: "tok"})
	require.NoError(t, err)

	since := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC)
	commits, err := src.CommitsBy(context.Background(), "alice", since, until)
	require.NoError(t, err)
	require.Len(t, commits, 1)
	require.Equal(t, "feat: thing", commits[0].Title)
	require.Equal(t, "team/project", commits[0].Project)
	require.Contains(t, commits[0].URL, "/team/project/-/commit/abc1234567def")
}

func TestHTTPSource_CommitsBy_SkipsCommitsWithUnresolvedProject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v4/users") && r.URL.Query().Get("username") == "alice":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id": 42}]`))
		case strings.HasPrefix(r.URL.Path, "/api/v4/users/42/events"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[
				{"created_at":"2026-04-26T10:30:00Z","action_name":"pushed to","project_id":7,
				 "push_data":{"commit_to":"abc1234567def","commit_title":"feat: thing","ref":"main"}}
			]`))
		case strings.HasPrefix(r.URL.Path, "/api/v4/projects/7"):
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	src, err := NewHTTPSource(Config{BaseURL: srv.URL, Token: "tok"})
	require.NoError(t, err)

	since := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC)
	commits, err := src.CommitsBy(context.Background(), "alice", since, until)
	require.NoError(t, err)
	require.Empty(t, commits)
}

func TestHTTPSource_CommitsBy_AuthFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	src, err := NewHTTPSource(Config{BaseURL: srv.URL, Token: "bad"})
	require.NoError(t, err)
	_, err = src.CommitsBy(context.Background(), "alice", time.Now().Add(-24*time.Hour), time.Now())
	require.Error(t, err)
}
