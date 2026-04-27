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
	mux.HandleFunc("GET /api/todos/{id}/history", handleHistoryTodo(d))

	mux.HandleFunc("GET /api/tags", handleListTags(d))
	mux.HandleFunc("GET /api/events", handleEvents(d))

	mux.HandleFunc("POST /api/timer/start", handleTimerStart(d))
	mux.HandleFunc("POST /api/timer/stop", handleTimerStop(d))
	mux.HandleFunc("GET /api/timer/active", handleTimerActive(d))
	mux.HandleFunc("GET /api/todos/{id}/time", handleTodoTime(d))
	mux.HandleFunc("GET /api/todos/{id}/sessions", handleTodoTimerSessions(d))

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
