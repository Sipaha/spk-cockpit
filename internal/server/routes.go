package server

import (
	"io"
	"io/fs"
	"net/http"
	"strings"
	"time"

	webembed "github.com/spk/spk-cockpit/web/embed"
)

func registerRoutes(mux *http.ServeMux, d *Deps) {
	mux.HandleFunc("GET /api/health", handleHealth)

	mux.HandleFunc("GET /api/todos", handleListTodos(d))
	mux.HandleFunc("POST /api/todos", handleCreateTodo(d))
	mux.HandleFunc("GET /api/todos/{id}", handleGetTodo(d))
	mux.HandleFunc("PATCH /api/todos/{id}", handleUpdateTodo(d))
	mux.HandleFunc("DELETE /api/todos/{id}", handleDeleteTodo(d))
	mux.HandleFunc("POST /api/todos/{id}/restore", handleRestoreTodo(d))
	mux.HandleFunc("GET /api/todos/deleted", handleListDeletedTodos(d))
	mux.HandleFunc("GET /api/todos/{id}/history", handleHistoryTodo(d))

	mux.HandleFunc("GET /api/tags", handleListTags(d))
	mux.HandleFunc("PUT /api/tags/{name}", handleUpsertTag(d))
	mux.HandleFunc("DELETE /api/tags/{name}", handleDeleteTag(d))
	mux.HandleFunc("GET /api/events", handleEvents(d))

	mux.HandleFunc("POST /api/timer/start", handleTimerStart(d))
	mux.HandleFunc("POST /api/timer/stop", handleTimerStop(d))
	mux.HandleFunc("GET /api/timer/active", handleTimerActive(d))
	mux.HandleFunc("GET /api/todos/{id}/time", handleTodoTime(d))
	mux.HandleFunc("GET /api/todos/{id}/sessions", handleTodoTimerSessions(d))

	mux.HandleFunc("GET /api/meetings", handleListMeetings(d))
	mux.HandleFunc("POST /api/meetings", handleCreateMeeting(d))
	mux.HandleFunc("GET /api/meetings/next", handleNextMeeting(d))
	mux.HandleFunc("GET /api/meetings/{id}", handleGetMeeting(d))
	mux.HandleFunc("PATCH /api/meetings/{id}", handleUpdateMeeting(d))
	mux.HandleFunc("DELETE /api/meetings/{id}", handleDeleteMeeting(d))
	mux.HandleFunc("GET /api/meetings/{id}/note", handleNoteForMeeting(d))

	mux.HandleFunc("PUT /api/notes", handleUpsertNote(d))
	mux.HandleFunc("DELETE /api/notes/{id}", handleDeleteNote(d))
	mux.HandleFunc("GET /api/todos/{id}/note", handleNoteForTodo(d))

	mux.HandleFunc("GET /api/secrets", handleListSecrets(d))
	mux.HandleFunc("PUT /api/secrets/{name}", handleSetSecret(d))
	mux.HandleFunc("DELETE /api/secrets/{name}", handleDeleteSecret(d))

	mux.HandleFunc("POST /api/sync/{source}", handleSyncTrigger(d))
	mux.HandleFunc("GET /api/sync", handleSyncStatus(d))

	mux.HandleFunc("GET /api/standup", handleStandup(d))

	mux.HandleFunc("GET /api/kv/{key}", handleGetKv(d))
	mux.HandleFunc("PUT /api/kv/{key}", handleSetKv(d))

	if dist, err := fs.Sub(webembed.DistFS, "dist"); err == nil {
		mux.Handle("/", spaFallback(dist))
	}
}

func spaFallback(dist fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(dist))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Strip leading slash for fs.Open
		clean := strings.TrimPrefix(r.URL.Path, "/")
		if clean == "" {
			clean = "index.html"
		}
		if f, err := dist.Open(clean); err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		// Fall back to index.html for SPA routing
		idx, err := dist.Open("index.html")
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		defer func() { _ = idx.Close() }()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if seeker, ok := idx.(io.ReadSeeker); ok {
			http.ServeContent(w, r, "index.html", time.Time{}, seeker)
			return
		}
		_, _ = io.Copy(w, idx)
	})
}
