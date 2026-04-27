package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/meeting"
)

// handleListMeetings returns a filtered list of meetings.
func handleListMeetings(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f := meeting.MeetingFilter{}
		q := r.URL.Query()
		if v := q.Get("from"); v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				f.FromUnix = n
			}
		}
		if v := q.Get("to"); v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				f.ToUnix = n
			}
		}
		if v := q.Get("includeCancelled"); v == "1" || v == "true" {
			f.IncludeDone = true
		}
		if v := q.Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				f.Limit = n
			}
		}
		list, err := d.Meetings.List(r.Context(), f)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "meeting.list_failed", err.Error())
			return
		}
		if list == nil {
			list = []api.Meeting{}
		}
		writeJSON(w, http.StatusOK, list)
	}
}

// handleNextMeeting returns the earliest upcoming non-cancelled meeting, or null.
func handleNextMeeting(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		next, err := d.Meetings.Next(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "meeting.next_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, next)
	}
}

// handleCreateMeeting creates a new manual meeting.
func handleCreateMeeting(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req api.CreateMeetingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		m, err := d.Meetings.CreateManual(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, "meeting.create_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, m)
	}
}

// handleGetMeeting returns a meeting by ID.
func handleGetMeeting(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		m, err := d.Meetings.Get(r.Context(), id)
		if errors.Is(err, meeting.ErrNotFound) {
			writeError(w, http.StatusNotFound, "meeting.not_found", "not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "meeting.get_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, m)
	}
}

// handleUpdateMeeting patches a manual meeting by ID.
func handleUpdateMeeting(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var req api.UpdateMeetingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		m, err := d.Meetings.UpdateManual(r.Context(), id, req)
		if errors.Is(err, meeting.ErrNotFound) {
			writeError(w, http.StatusNotFound, "meeting.not_found", "not found")
			return
		}
		if errors.Is(err, meeting.ErrManualOnly) {
			writeError(w, http.StatusForbidden, "meeting.manual_only", "only manual meetings may be edited")
			return
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "meeting.update_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, m)
	}
}

// handleDeleteMeeting soft-deletes a manual meeting by ID.
func handleDeleteMeeting(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		err := d.Meetings.DeleteManual(r.Context(), id)
		if errors.Is(err, meeting.ErrNotFound) {
			writeError(w, http.StatusNotFound, "meeting.not_found", "not found")
			return
		}
		if errors.Is(err, meeting.ErrManualOnly) {
			writeError(w, http.StatusForbidden, "meeting.manual_only", "only manual meetings may be deleted")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "meeting.delete_failed", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleSyncTrigger forces an immediate CalDAV sync for the named source.
func handleSyncTrigger(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		source := r.PathValue("source")
		if d.Sync == nil {
			writeError(w, http.StatusServiceUnavailable, "sync.disabled", "sync is not configured")
			return
		}
		if err := d.Sync.TriggerNow(source); err != nil {
			writeError(w, http.StatusBadRequest, "sync.trigger_failed", err.Error())
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}
}

// handleSyncStatus returns the current sync state for all configured sources.
func handleSyncStatus(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if d.Sync == nil {
			writeJSON(w, http.StatusOK, []api.SyncStateEntry{})
			return
		}
		writeJSON(w, http.StatusOK, d.Sync.Status())
	}
}
