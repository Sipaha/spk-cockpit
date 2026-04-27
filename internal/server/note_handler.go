package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/note"
)

// handleUpsertNote creates or updates the note attached to a meeting or todo.
func handleUpsertNote(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			MeetingID string `json:"meetingId,omitempty"`
			TodoID    string `json:"todoId,omitempty"`
			Body      string `json:"body"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		n, err := d.Notes.Upsert(r.Context(), apiUpsertNoteFromHandler(req.MeetingID, req.TodoID, req.Body))
		if err != nil {
			writeError(w, http.StatusBadRequest, "note.upsert_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, n)
	}
}

// handleDeleteNote soft-deletes a note by ID.
func handleDeleteNote(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		err := d.Notes.Delete(r.Context(), id)
		if errors.Is(err, note.ErrNotFound) {
			writeError(w, http.StatusNotFound, "note.not_found", "not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "note.delete_failed", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleNoteForMeeting returns the note attached to a meeting, or null if absent.
func handleNoteForMeeting(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		got, err := d.Notes.FindByMeeting(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "note.lookup_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, got)
	}
}

// handleNoteForTodo returns the note attached to a todo, or null if absent.
func handleNoteForTodo(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		got, err := d.Notes.FindByTodo(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "note.lookup_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, got)
	}
}

// apiUpsertNoteFromHandler constructs an api.UpsertNoteRequest from handler fields.
func apiUpsertNoteFromHandler(meetingID, todoID, body string) api.UpsertNoteRequest {
	return api.UpsertNoteRequest{MeetingID: meetingID, TodoID: todoID, Body: body}
}
