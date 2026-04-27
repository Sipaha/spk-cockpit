package server

import "net/http"

func registerRoutes(mux *http.ServeMux, d *Deps) {
	mux.HandleFunc("GET /api/health", handleHealth)

	mux.HandleFunc("GET /api/todos", handleListTodos(d))
	mux.HandleFunc("POST /api/todos", handleCreateTodo(d))
	mux.HandleFunc("GET /api/todos/{id}", handleGetTodo(d))
	mux.HandleFunc("PATCH /api/todos/{id}", handleUpdateTodo(d))
	mux.HandleFunc("DELETE /api/todos/{id}", handleDeleteTodo(d))
	mux.HandleFunc("GET /api/todos/{id}/history", handleHistoryTodo(d))

	mux.HandleFunc("GET /api/tags", handleListTags(d))
}
