package server

import (
	"net/http"
	"time"
)

// handleStandup serves GET /api/standup?date=YYYY-MM-DD.
// If date is omitted, today's local date is used.
func handleStandup(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if d.Standup == nil {
			writeError(w, http.StatusServiceUnavailable, "standup.unavailable", "standup service not configured")
			return
		}
		day := time.Now()
		if q := r.URL.Query().Get("date"); q != "" {
			parsed, err := time.ParseInLocation("2006-01-02", q, time.Local)
			if err != nil {
				writeError(w, http.StatusBadRequest, "standup.bad_date", "invalid date (expected YYYY-MM-DD)")
				return
			}
			day = parsed
		}
		report, err := d.Standup.Generate(r.Context(), day)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "standup.generate_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, report)
	}
}
