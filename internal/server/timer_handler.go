package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/timer"
)

func handleTimerStart(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req api.StartTimerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		if req.TodoID == "" {
			writeError(w, http.StatusBadRequest, "bad_request", "todoId is required")
			return
		}
		s, err := d.Timer.Start(r.Context(), req.TodoID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "timer.start_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, s)
	}
}

func handleTimerStop(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, _, err := d.Timer.Stop(r.Context())
		if errors.Is(err, timer.ErrNoActiveSession) {
			writeError(w, http.StatusConflict, "timer.no_active", "no active session")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "timer.stop_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, s)
	}
}

func handleTimerActive(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		active, err := d.Timer.Active(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "timer.active_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, active)
	}
}

func handleTodoTime(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var since int64
		if v := r.URL.Query().Get("since"); v != "" {
			n, err := strconv.ParseInt(v, 10, 64)
			if err == nil {
				since = n
			}
		}
		total, err := d.Timer.TotalForTodo(r.Context(), id, since)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "timer.total_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, total)
	}
}

func handleTodoTimerSessions(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		limit := 100
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil { //nolint:gosec
				limit = n
			}
		}
		sessions, err := d.Timer.ListSessions(r.Context(), id, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "timer.list_failed", err.Error())
			return
		}
		if sessions == nil {
			sessions = []api.TimerSession{}
		}
		writeJSON(w, http.StatusOK, sessions)
	}
}
