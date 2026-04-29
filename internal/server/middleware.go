package server

import (
	"log/slog"
	"net/http"
	"time"
)

// maxBodyBytes caps the size of any single request body the API will accept.
// SSE responses don't have a request body so this doesn't affect them.
const maxBodyBytes = 1 << 20 // 1 MiB

// maxBodyMW wraps r.Body with http.MaxBytesReader so a single oversized payload
// can't exhaust process memory. SSE handlers ignore r.Body, so streaming is
// unaffected. We always wrap when Body is non-nil — including chunked
// (ContentLength == -1) requests, where skipping the wrap would let a malicious
// caller send unbounded chunked bodies.
func maxBodyMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		}
		next.ServeHTTP(w, r)
	})
}

// recoverMW catches panics in handlers, logs them, and returns a 500.
func recoverMW(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rv := recover(); rv != nil {
				log.Error("panic in handler", "path", r.URL.Path, "value", rv)
				writeError(w, http.StatusInternalServerError, "internal", "internal error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// requestLog logs each request at debug level.
func requestLog(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(rw, r)
		log.Debug("request", "method", r.Method, "path", r.URL.Path, "status", rw.status, "dur_ms", time.Since(start).Milliseconds())
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (sw *statusWriter) WriteHeader(c int) {
	sw.status = c
	sw.ResponseWriter.WriteHeader(c)
}

func (sw *statusWriter) Flush() {
	if f, ok := sw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
