package server

import "net/http"

func registerRoutes(mux *http.ServeMux, _ *Deps) {
	mux.HandleFunc("GET /api/health", handleHealth)
}
