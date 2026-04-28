package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/todo"
)

func handleListTodos(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f := todo.TodoFilter{}
		q := r.URL.Query()
		if v := q.Get("includeDone"); v == "1" || v == "true" {
			f.IncludeDone = true
		}
		if v := q.Get("search"); v != "" {
			f.Search = v
		}
		for _, s := range q["status"] {
			f.Statuses = append(f.Statuses, api.TodoStatus(s))
		}
		for _, p := range q["priority"] {
			n, err := strconv.Atoi(p)
			if err == nil {
				f.Priorities = append(f.Priorities, api.Priority(n))
			}
		}
		for _, t := range q["tag"] {
			if t != "" {
				f.Tags = append(f.Tags, t)
			}
		}
		if v := q.Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				f.Limit = n
			}
		}
		list, err := d.Todos.List(r.Context(), f)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "todo.list_failed", err.Error())
			return
		}
		if list == nil {
			list = []api.Todo{}
		}
		writeJSON(w, http.StatusOK, list)
	}
}

func handleCreateTodo(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req api.CreateTodoRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		t, err := d.Todos.Create(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, "todo.create_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, t)
	}
}

func handleGetTodo(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		t, err := d.Todos.Get(r.Context(), id)
		if errors.Is(err, todo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "todo.not_found", "todo not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "todo.get_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, t)
	}
}

func handleUpdateTodo(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var req api.UpdateTodoRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}

		// Snapshot the previous status so the auto-timer hook below can tell
		// whether this PATCH is an in_progress transition. We can't infer it
		// from the response alone — req.Status may be a no-op for the same
		// status the todo already had.
		var oldStatus api.TodoStatus
		if req.Status != nil {
			if prev, err := d.Todos.Get(r.Context(), id); err == nil {
				oldStatus = prev.Status
			}
		}

		t, err := d.Todos.Update(r.Context(), id, req)
		if errors.Is(err, todo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "todo.not_found", "todo not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "todo.update_failed", err.Error())
			return
		}

		// Auto-timer: dragging a card into "In Progress" should start tracking
		// time on it; pulling it out should stop. We only stop when the active
		// session belongs to *this* todo so a manually-running timer for some
		// other task isn't yanked when the user moves an unrelated card.
		if d.Timer != nil && req.Status != nil && oldStatus != t.Status {
			ctx := r.Context()
			switch {
			case t.Status == api.StatusInProgress:
				if _, err := d.Timer.Start(ctx, t.ID); err != nil {
					// Logged at the layer above; don't fail the PATCH for it.
					_ = err
				}
			case oldStatus == api.StatusInProgress:
				if active, err := d.Timer.Active(ctx); err == nil && active != nil && active.TodoID == t.ID {
					_, _, _ = d.Timer.Stop(ctx)
				}
			}
		}

		writeJSON(w, http.StatusOK, t)
	}
}

func handleDeleteTodo(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		err := d.Todos.Delete(r.Context(), id)
		if errors.Is(err, todo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "todo.not_found", "todo not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "todo.delete_failed", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleHistoryTodo(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		limit := 100
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				limit = n
			}
		}
		events, err := d.Todos.History(r.Context(), id, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "todo.history_failed", err.Error())
			return
		}
		if events == nil {
			events = []api.TodoEvent{}
		}
		writeJSON(w, http.StatusOK, events)
	}
}

func handleListTags(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tags, err := d.Tags.List(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "tag.list_failed", err.Error())
			return
		}
		if tags == nil {
			tags = []api.Tag{}
		}
		writeJSON(w, http.StatusOK, tags)
	}
}

func handleUpsertTag(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimSpace(r.PathValue("name"))
		if name == "" {
			writeError(w, http.StatusBadRequest, "bad_request", "tag name required")
			return
		}
		var req api.UpsertTagRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		t := api.Tag{Name: name, Color: req.Color, CreatedAt: time.Now().Unix()}
		if err := d.Tags.Upsert(r.Context(), t); err != nil {
			writeError(w, http.StatusInternalServerError, "tag.upsert_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, t)
	}
}

func handleDeleteTag(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimSpace(r.PathValue("name"))
		if name == "" {
			writeError(w, http.StatusBadRequest, "bad_request", "tag name required")
			return
		}
		if err := d.Tags.Delete(r.Context(), name); err != nil {
			writeError(w, http.StatusInternalServerError, "tag.delete_failed", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
