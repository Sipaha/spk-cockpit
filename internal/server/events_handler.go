package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/spk/spk-cockpit/internal/api"
)

func handleEvents(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if d.Bus == nil {
			writeError(w, http.StatusServiceUnavailable, "bus_unavailable", "event bus not initialized")
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			writeError(w, http.StatusInternalServerError, "no_flusher", "streaming unsupported")
			return
		}

		// Subscribe BEFORE flushing the response headers. Flushing causes the
		// client's Do() to return, after which the test (or a real client) may
		// immediately publish an event — if we subscribed after the flush, we'd
		// race the publish and miss the event entirely.
		ch := d.Bus.Subscribe(64)
		defer d.Bus.Unsubscribe(ch)

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case evt, ok := <-ch:
				if !ok {
					return
				}
				b, err := json.Marshal(api.Event{Type: evt.Type, Data: evt.Data})
				if err != nil {
					continue
				}
				if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
					return
				}
				flusher.Flush()
			}
		}
	}
}
