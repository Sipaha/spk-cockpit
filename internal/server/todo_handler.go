package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

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

		// Auto-timer: dragging a card into "In Progress" starts a timer on it;
		// pulling it out stops only that todo's session. Sibling timers on
		// other in-progress cards are not touched, so parallel work-in-flight
		// stays running.
		if d.Timer != nil && req.Status != nil && oldStatus != t.Status {
			ctx := r.Context()
			switch {
			case t.Status == api.StatusInProgress:
				_, _ = d.Timer.Start(ctx, t.ID)
			case oldStatus == api.StatusInProgress:
				if _, _, err := d.Timer.Stop(ctx, t.ID); err != nil && !errors.Is(err, timer.ErrNoActiveSession) {
					_ = err
				}
			}
		}

		writeJSON(w, http.StatusOK, t)
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

		// Snapshot the previous status so the auto-timer hook below can
		// react to a transition. Same shape as in handleUpdateTodo.
		var oldStatus api.TodoStatus
		if req.Status != nil {
			if prev, err := d.Todos.Get(r.Context(), id); err == nil {
				oldStatus = prev.Status
			}
		}

		t, err := d.Todos.Move(r.Context(), id, req)
		if errors.Is(err, todo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "todo.not_found", "todo not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "todo.move_failed", err.Error())
			return
		}

		// Auto-timer mirrors the PATCH path — start when entering In Progress,
		// stop when leaving (only the moved todo's session, never siblings').
		if d.Timer != nil && req.Status != nil && oldStatus != t.Status {
			ctx := r.Context()
			switch {
			case t.Status == api.StatusInProgress:
				_, _ = d.Timer.Start(ctx, t.ID)
			case oldStatus == api.StatusInProgress:
				if _, _, err := d.Timer.Stop(ctx, t.ID); err != nil && !errors.Is(err, timer.ErrNoActiveSession) {
					_ = err
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

func handleRenameTag(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		oldName := strings.TrimSpace(r.PathValue("name"))
		if oldName == "" {
			writeError(w, http.StatusBadRequest, "bad_request", "tag name required")
			return
		}
		var req api.RenameTagRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		newName := strings.TrimSpace(req.NewName)
		if newName == "" {
			writeError(w, http.StatusBadRequest, "bad_request", "newName required")
			return
		}
		if err := d.Tags.Rename(r.Context(), oldName, newName); err != nil {
			writeError(w, http.StatusBadRequest, "tag.rename_failed", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
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
