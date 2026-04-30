package server

import (
	"net/http"
)

func registerRoutes(mux *http.ServeMux, d *Deps) {
	mux.HandleFunc("GET /api/health", handleHealth)

	mux.HandleFunc("GET /api/todos", handleListTodos(d))
	mux.HandleFunc("POST /api/todos", handleCreateTodo(d))
	mux.HandleFunc("GET /api/todos/{id}", handleGetTodo(d))
	mux.HandleFunc("PATCH /api/todos/{id}", handleUpdateTodo(d))
	mux.HandleFunc("POST /api/todos/{id}/move", handleMoveTodo(d))
	mux.HandleFunc("DELETE /api/todos/{id}", handleDeleteTodo(d))
	mux.HandleFunc("POST /api/todos/{id}/restore", handleRestoreTodo(d))
	mux.HandleFunc("POST /api/todos/{id}/dismiss", handleDismissTodo(d))
	mux.HandleFunc("GET /api/todos/deleted", handleListDeletedTodos(d))

	mux.HandleFunc("GET /api/tags", handleListTags(d))
	mux.HandleFunc("PUT /api/tags/{name}", handleUpsertTag(d))
	mux.HandleFunc("POST /api/tags/{name}/rename", handleRenameTag(d))
	mux.HandleFunc("DELETE /api/tags/{name}", handleDeleteTag(d))
	mux.HandleFunc("GET /api/events", handleEvents(d))

	mux.HandleFunc("POST /api/timer/start", handleTimerStart(d))
	mux.HandleFunc("POST /api/timer/stop", handleTimerStop(d))
	mux.HandleFunc("GET /api/timer/active", handleTimerActive(d))

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

	mux.HandleFunc("GET /api/kv/{key}", handleGetKv(d))
	mux.HandleFunc("PUT /api/kv/{key}", handleSetKv(d))
}
