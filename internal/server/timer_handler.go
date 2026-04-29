package server

import (
	"encoding/json"
	"errors"
	"net/http"

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
		var req api.StopTimerRequest
		// Empty body is fine — it means "stop all". The tray "Stop timer"
		// action uses that path; the per-card stop button always sends a
		// todoId so it never affects sibling timers.
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.TodoID == "" {
			active, err := d.Timer.Active(r.Context())
			if err != nil {
				writeError(w, http.StatusInternalServerError, "timer.active_failed", err.Error())
				return
			}
			stopped := make([]api.TimerSession, 0, len(active))
			for _, a := range active {
				s, _, err := d.Timer.Stop(r.Context(), a.TodoID)
				if err != nil && !errors.Is(err, timer.ErrNoActiveSession) {
					writeError(w, http.StatusInternalServerError, "timer.stop_failed", err.Error())
					return
				}
				if err == nil {
					stopped = append(stopped, s)
				}
			}
			writeJSON(w, http.StatusOK, stopped)
			return
		}
		s, _, err := d.Timer.Stop(r.Context(), req.TodoID)
		if errors.Is(err, timer.ErrNoActiveSession) {
			writeError(w, http.StatusConflict, "timer.no_active", "no active session for todo")
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
