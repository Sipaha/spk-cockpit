package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/secret"
)

// handleListSecrets returns the names of all stored secrets (no values).
func handleListSecrets(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		names, err := d.Secrets.ListNames(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "secret.list_failed", err.Error())
			return
		}
		out := make([]api.Secret, 0, len(names))
		for _, n := range names {
			out = append(out, api.Secret{Name: n})
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// handleSetSecret encrypts and stores a secret value under the given name.
func handleSetSecret(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		var req api.SetSecretRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		if err := d.Secrets.Set(r.Context(), name, req.Value); err != nil {
			writeError(w, http.StatusInternalServerError, "secret.set_failed", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleDeleteSecret removes a secret by name.
func handleDeleteSecret(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if err := d.Secrets.Delete(r.Context(), name); err != nil {
			if errors.Is(err, secret.ErrNotFound) {
				writeError(w, http.StatusNotFound, "secret.not_found", "not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "secret.delete_failed", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
