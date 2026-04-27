package server

import (
	"encoding/json"
	"net/http"
)

// handleGetKv retrieves a key-value entry; value is null when the key is absent.
func handleGetKv(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("key")
		v, ok, err := d.Kv.Get(r.Context(), key)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "kv.get_failed", err.Error())
			return
		}
		var val *string
		if ok {
			val = &v
		}
		writeJSON(w, http.StatusOK, map[string]any{"key": key, "value": val})
	}
}

// handleSetKv upserts a key-value entry.
func handleSetKv(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("key")
		var body struct {
			Value string `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		if err := d.Kv.Set(r.Context(), key, body.Value); err != nil {
			writeError(w, http.StatusInternalServerError, "kv.set_failed", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
