// Package server hosts an HTTP+SSE API on a Unix Domain Socket. The same routes
// serve the React UI (proxied by Wails) and the CLI subcommands (over UDS).
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/eventbus"
	"github.com/spk/spk-cockpit/internal/meeting"
	"github.com/spk/spk-cockpit/internal/note"
	"github.com/spk/spk-cockpit/internal/secret"
	"github.com/spk/spk-cockpit/internal/standup"
	"github.com/spk/spk-cockpit/internal/timer"
	"github.com/spk/spk-cockpit/internal/todo"
)

// Config configures a Server.
type Config struct {
	// SocketPath is the path to the listening UDS file. Required.
	SocketPath string
	// Logger optional; defaults to slog.Default().
	Logger *slog.Logger
}

// Deps wires domain services to HTTP handlers. Fields are filled by callers between New() and Serve().
type Deps struct {
	Todos    *todo.Service
	Tags     todo.TagRepo
	Bus      *eventbus.Bus
	Timer    *timer.Service
	Meetings *meeting.Service
	Notes    *note.Service
	Secrets  *secret.Service
	Sync     SyncTrigger
	Kv       todo.KvRepo
	Standup  *standup.Service
}

// SyncTrigger lets the server force a CalDAV sync from a CLI/UI request.
type SyncTrigger interface {
	TriggerNow(source string) error
	Status() []api.SyncStateEntry
}

// Server owns the UDS listener and HTTP server.
type Server struct {
	cfg      Config
	listener net.Listener
	httpSrv  *http.Server
	logger   *slog.Logger
	deps     *Deps
}

// New binds to SocketPath, removes a stale socket if present, and chmods 0600. Routes are registered when Serve is called.
func New(cfg Config) (*Server, error) {
	if cfg.SocketPath == "" {
		return nil, errors.New("server: SocketPath is required")
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	if err := removeStaleSocket(cfg.SocketPath); err != nil {
		return nil, fmt.Errorf("remove stale socket: %w", err)
	}
	ln, err := net.Listen("unix", cfg.SocketPath)
	if err != nil {
		return nil, fmt.Errorf("listen unix: %w", err)
	}
	if err := os.Chmod(cfg.SocketPath, 0o600); err != nil {
		_ = ln.Close()
		return nil, fmt.Errorf("chmod socket: %w", err)
	}
	return &Server{cfg: cfg, listener: ln, logger: logger, deps: &Deps{}}, nil
}

// Serve registers routes and serves until Stop is called.
func (s *Server) Serve() error {
	mux := http.NewServeMux()
	registerRoutes(mux, s.deps)
	s.httpSrv = &http.Server{
		Handler:           recoverMW(s.logger, requestLog(s.logger, mux)),
		ReadHeaderTimeout: 5 * time.Second,
	}
	err := s.httpSrv.Serve(s.listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Stop gracefully shuts down the HTTP server.
func (s *Server) Stop(ctx context.Context) error {
	if s.httpSrv == nil {
		return nil
	}
	shutdownCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return s.httpSrv.Shutdown(shutdownCtx)
}

// Deps exposes the dependency struct for callers to populate between New() and Serve().
func (s *Server) Deps() *Deps { return s.deps }

func removeStaleSocket(path string) error {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	c, err := net.DialTimeout("unix", path, 200*time.Millisecond)
	if err == nil {
		_ = c.Close()
		return fmt.Errorf("socket %s already in use", path)
	}
	return os.Remove(path)
}
