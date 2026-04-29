package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/timer"
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
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
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

		// Service.Update returns the pre-mutation status atomically with the
		// row update — no separate pre-flight Get, so no TOCTOU window.
		t, oldStatus, err := d.Todos.Update(r.Context(), id, req)
		if errors.Is(err, todo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "todo.not_found", "todo not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "todo.update_failed", err.Error())
			return
		}

		applyAutoTimer(r.Context(), d, req.Status, oldStatus, t)
		writeJSON(w, http.StatusOK, t)
	}
}

// applyAutoTimer is the shared auto-timer hook: dragging a card into
// "In Progress" starts a timer on it; pulling it out stops only that todo's
// session. Sibling timers on other in-progress cards are never touched.
// statusReq carries the original PATCH/move request's Status field — nil means
// the caller didn't ask for a status change so the hook is a no-op.
//
// We detach from the request context so a client disconnect after the row
// update commits doesn't leave the UI showing in_progress with no running
// timer. The detached context still carries values from r.Context() (slog,
// trace), just not its cancellation signal.
func applyAutoTimer(ctx context.Context, d *Deps, statusReq *api.TodoStatus, oldStatus api.TodoStatus, t api.Todo) {
	if d.Timer == nil || statusReq == nil || oldStatus == t.Status {
		return
	}
	bg := context.WithoutCancel(ctx)
	switch {
	case t.Status == api.StatusInProgress:
		_, _ = d.Timer.Start(bg, t.ID)
	case oldStatus == api.StatusInProgress:
		if _, _, err := d.Timer.Stop(bg, t.ID); err != nil && !errors.Is(err, timer.ErrNoActiveSession) {
			_ = err
		}
	}
}

func handleMoveTodo(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var req api.MoveTodoRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}

		// Service.Move returns the pre-mutation status atomically with the
		// move — no separate pre-flight Get, so no TOCTOU window.
		t, oldStatus, err := d.Todos.Move(r.Context(), id, req)
		if errors.Is(err, todo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "todo.not_found", "todo not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "todo.move_failed", err.Error())
			return
		}

		applyAutoTimer(r.Context(), d, req.Status, oldStatus, t)
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

func handleDismissTodo(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		t, err := d.Todos.DismissDone(r.Context(), id)
		if errors.Is(err, todo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "todo.not_found", "todo not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "todo.dismiss_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, t)
	}
}

func handleRestoreTodo(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		t, err := d.Todos.Restore(r.Context(), id)
		if errors.Is(err, todo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "todo.not_found", "todo not found or not deleted")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "todo.restore_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, t)
	}
}

func handleListDeletedTodos(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := 100
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				limit = n
			}
		}
		list, err := d.Todos.ListDeleted(r.Context(), limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "todo.list_deleted_failed", err.Error())
			return
		}
		if list == nil {
			list = []api.Todo{}
		}
		writeJSON(w, http.StatusOK, list)
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
		var req api.UpsertTagRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		t, err := d.Tags.Upsert(r.Context(), r.PathValue("name"), req.Color)
		if errors.Is(err, todo.ErrTagNameRequired) {
			writeError(w, http.StatusBadRequest, "bad_request", "tag name required")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "tag.upsert_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, t)
	}
}

func handleRenameTag(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req api.RenameTagRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		err := d.Tags.Rename(r.Context(), r.PathValue("name"), req.NewName)
		if errors.Is(err, todo.ErrTagNameRequired) {
			writeError(w, http.StatusBadRequest, "bad_request", "tag name required")
			return
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "tag.rename_failed", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleDeleteTag(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := d.Tags.Delete(r.Context(), r.PathValue("name"))
		if errors.Is(err, todo.ErrTagNameRequired) {
			writeError(w, http.StatusBadRequest, "bad_request", "tag name required")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "tag.delete_failed", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
