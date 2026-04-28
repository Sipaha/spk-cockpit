# spk-cockpit Phase 1: Foundation + Todo Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Working tray-resident Go binary with embedded React UI that manages a personal todo list (CRUD, status, priority, tags, audit history), backed by SQLite, exposing both a full-window Wails UI and a basic CLI over a Unix Domain Socket.

**Architecture:** Single Go binary. Domain layer is pure (depends only on repository interfaces and a `Clock`). Storage is SQLite via `modernc.org/sqlite`. HTTP server on UDS exposes REST + SSE. Wails v2 wraps the embedded React UI in a webview that routes its `fetch()` over the same UDS. CLI subcommands talk to the running daemon over the same UDS.

**Tech Stack:** Go 1.22+, Cobra, modernc.org/sqlite, Wails v2, fyne.io/systray, testify, godbus (deferred to phase 3), React 19, Vite, TypeScript, Tailwind 4, Zustand, lucide-react, pnpm.

**Out of scope for phase 1:** popover window, time-tracking, meetings, notes, CalDAV, GitLab, Tracker, notifications, secrets encryption, autostart, distribution. These belong to phases 2–4.

**Reference codebase:** `/home/spk/IdeaProjects/reference-app/` — same Go-binary-with-embedded-React pattern. Read its `Makefile`, `cmd/main.go`, `internal/storage/`, `internal/daemon/`, `web/vite.config.ts` when in doubt about a specific pattern.

---

## File Structure (after Phase 1)

```
spk-task-manager/
├── go.mod, go.sum
├── Makefile
├── README.md
├── .gitignore
├── .golangci.yml
├── cmd/cockpit/main.go                       # entry point (cobra root)
├── internal/
│   ├── api/
│   │   ├── dto.go                            # Todo, Tag, TodoEvent DTOs
│   │   └── events.go                         # event types for SSE
│   ├── cli/
│   │   ├── root.go                           # cobra root + version
│   │   ├── start.go                          # `cockpit start`
│   │   ├── stop.go                           # `cockpit stop`
│   │   ├── todo.go                           # `cockpit todo add/list/done/rm`
│   │   └── client.go                         # UDS HTTP client for CLI
│   ├── clock/
│   │   ├── clock.go                          # Clock interface + realClock
│   │   └── fake.go                           # fakeClock for tests
│   ├── eventbus/
│   │   ├── bus.go                            # in-memory event bus
│   │   └── bus_test.go
│   ├── store/
│   │   ├── store.go                          # *sql.DB + open/close
│   │   ├── migrate.go                        # migration runner
│   │   ├── migrations/
│   │   │   └── 0001_init.sql
│   │   ├── todo_repo.go                      # SQLite TodoRepo
│   │   ├── tag_repo.go                       # SQLite TagRepo
│   │   ├── event_repo.go                     # SQLite EventRepo (todo_events)
│   │   ├── kv_repo.go                        # SQLite KvRepo
│   │   └── conformance_test.go               # contract tests
│   ├── todo/
│   │   ├── service.go                        # Todo domain service
│   │   ├── repo.go                           # TodoRepo / TagRepo / EventRepo interfaces
│   │   ├── service_test.go                   # unit tests with fake repos
│   │   └── fakerepo/
│   │       ├── todo_repo.go
│   │       ├── tag_repo.go
│   │       └── event_repo.go
│   ├── server/
│   │   ├── server.go                         # HTTP server on UDS
│   │   ├── routes.go                         # route registration
│   │   ├── todo_handler.go
│   │   ├── tag_handler.go
│   │   ├── events_handler.go                 # SSE
│   │   ├── health_handler.go
│   │   └── middleware.go                     # recovery, logging
│   ├── window/
│   │   └── window.go                         # Wails main window setup
│   ├── tray/
│   │   ├── tray.go                           # tray.Backend interface
│   │   └── tray_linux.go                     # fyne.io/systray impl
│   ├── paths/
│   │   └── paths.go                          # XDG paths
│   ├── log/
│   │   └── log.go                            # slog setup
│   └── appfiles/
│       └── icons.go                          # embedded tray icons
├── icons/
│   ├── tray.png                              # 22x22 tray icon
│   └── tray-error.png                        # red-dot error variant
└── web/
    ├── package.json
    ├── pnpm-lock.yaml
    ├── tsconfig.json
    ├── vite.config.ts
    ├── tailwind.config.js
    ├── postcss.config.js
    ├── index.html
    ├── eslint.config.js
    ├── public/
    └── src/
        ├── main.tsx
        ├── App.tsx
        ├── index.css                         # Tailwind directives
        ├── lib/
        │   ├── api.ts                        # REST client
        │   ├── events.ts                     # SSE EventSource wrapper
        │   ├── store.ts                      # Zustand todo store
        │   └── types.ts                      # TS mirror of Go DTOs
        ├── components/
        │   ├── TodoList.tsx
        │   ├── TodoRow.tsx
        │   ├── AddTodoForm.tsx
        │   └── TagPill.tsx
        └── pages/
            └── Todos.tsx
```

After phase 1, `web/dist` is built and embedded into the Go binary via `go:embed`. The binary serves React from the embedded FS over UDS, and Wails opens a webview pointed at the same.

---

## Conventions

- **Commit style:** conventional commits (`feat:`, `fix:`, `test:`, `chore:`, `refactor:`). One commit per task unless noted. Never include `Co-Authored-By` lines.
- **Go formatting:** `gofmt` only. Tabs for indentation.
- **Imports:** stdlib first, third-party second, local third — separated by blank lines.
- **Errors:** wrap with `%w`. Domain layer returns sentinel errors (`todo.ErrNotFound`).
- **SQLite connection:** every connection opens with `PRAGMA foreign_keys=ON; PRAGMA journal_mode=WAL; PRAGMA synchronous=NORMAL;`.
- **IDs:** ULID via `github.com/oklog/ulid/v2` (lexicographic time order, 26 chars).
- **Paths:** XDG-compliant via `internal/paths`. Override via env vars `SPK_COCKPIT_DATA_DIR`, `SPK_COCKPIT_STATE_DIR`, `SPK_COCKPIT_CONFIG_DIR` for tests.

---

## Task 1: Initialize project skeleton

**Files:**
- Create: `go.mod`
- Create: `.gitignore`
- Create: `README.md`
- Create: `Makefile`
- Create: `.golangci.yml`
- Create: `cmd/cockpit/main.go`

- [ ] **Step 1.1: Initialize git and Go module**

```bash
cd /home/spk/IdeaProjects/spk-task-manager
git init -b main
go mod init github.com/spk/spk-cockpit
```

- [ ] **Step 1.2: Write `.gitignore`**

```gitignore
# Binaries
build/
*.exe
*.out

# Go
*.test
coverage.out
coverage.html

# Web
node_modules/
web/dist/
web/test-results/
.vite/

# Editors
.idea/
.vscode/
.DS_Store

# Local state
*.db
*.db.bak.*
*.sock
*.log
```

- [ ] **Step 1.3: Write minimal `cmd/cockpit/main.go`**

```go
package main

import (
	"fmt"
	"os"

	"github.com/spk/spk-cockpit/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 1.4: Write `internal/cli/root.go` (stub)**

```go
package cli

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:           "cockpit",
	Short:         "spk-cockpit — personal productivity tray app",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() error {
	return rootCmd.Execute()
}
```

- [ ] **Step 1.5: Write `Makefile`**

```makefile
.PHONY: build build-fast web-build test test-unit lint fmt tidy clean run

GO ?= go
BUILD_DIR := build/bin
BIN := $(BUILD_DIR)/spk-cockpit

build: web-build build-fast

build-fast:
	@mkdir -p $(BUILD_DIR)
	$(GO) build -trimpath -o $(BIN) ./cmd/cockpit

web-build:
	cd web && pnpm install --frozen-lockfile && pnpm build

test: test-unit
	cd web && pnpm test --run

test-unit:
	$(GO) test ./internal/...

lint:
	golangci-lint run
	cd web && pnpm lint

fmt:
	$(GO) fmt ./...

tidy:
	$(GO) mod tidy

clean:
	rm -rf $(BUILD_DIR) web/dist

run: build-fast
	$(BIN) start --foreground
```

- [ ] **Step 1.6: Write `.golangci.yml` (minimal)**

```yaml
version: "2"
run:
  timeout: 3m
linters:
  default: standard
  enable:
    - errorlint
    - gocritic
    - gosec
    - misspell
    - revive
    - staticcheck
    - unparam
    - unused
formatters:
  enable:
    - gofmt
    - goimports
```

- [ ] **Step 1.7: Write minimal `README.md`**

```markdown
# spk-cockpit

Personal productivity tray app — todo list, time-tracking, meetings, standup helper. Single Go binary with embedded React UI.

## Status

Phase 1 (foundation + todo). See `docs/superpowers/plans/` for the implementation plan.

## Build

```bash
make build           # full build (Go + React)
make test            # all tests
./build/bin/spk-cockpit start --foreground
```
```

- [ ] **Step 1.8: Add cobra dependency and verify build**

```bash
go get github.com/spf13/cobra
go mod tidy
go build ./cmd/cockpit
./cockpit --help
```

Expected output: shows `cockpit` short description and `--help`/`--version` flags. Delete the `cockpit` artifact afterwards (`rm cockpit`).

- [ ] **Step 1.9: Commit**

```bash
git add .
git commit -m "chore: initialize spk-cockpit project skeleton"
```

---

## Task 2: Set up XDG paths and slog

**Files:**
- Create: `internal/paths/paths.go`
- Create: `internal/paths/paths_test.go`
- Create: `internal/log/log.go`

- [ ] **Step 2.1: Write the failing test for paths**

```go
package paths

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPaths_DefaultsRespectXDG(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(tmp, "state"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "config"))
	t.Setenv("SPK_COCKPIT_DATA_DIR", "")
	t.Setenv("SPK_COCKPIT_STATE_DIR", "")
	t.Setenv("SPK_COCKPIT_CONFIG_DIR", "")

	p, err := New()
	require.NoError(t, err)

	require.Equal(t, filepath.Join(tmp, "data", "spk-cockpit"), p.DataDir)
	require.Equal(t, filepath.Join(tmp, "state", "spk-cockpit"), p.StateDir)
	require.Equal(t, filepath.Join(tmp, "config", "spk-cockpit"), p.ConfigDir)
	require.Equal(t, filepath.Join(p.DataDir, "cockpit.db"), p.DBFile)
	require.Equal(t, filepath.Join(p.StateDir, "cockpit.sock"), p.SocketFile)

	for _, d := range []string{p.DataDir, p.StateDir, p.ConfigDir} {
		_, err := os.Stat(d)
		require.NoError(t, err, "directory %s should exist", d)
	}
}

func TestPaths_EnvOverridesXDG(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("SPK_COCKPIT_DATA_DIR", filepath.Join(tmp, "custom-data"))
	t.Setenv("SPK_COCKPIT_STATE_DIR", filepath.Join(tmp, "custom-state"))
	t.Setenv("SPK_COCKPIT_CONFIG_DIR", filepath.Join(tmp, "custom-config"))

	p, err := New()
	require.NoError(t, err)

	require.Equal(t, filepath.Join(tmp, "custom-data"), p.DataDir)
	require.Equal(t, filepath.Join(tmp, "custom-state"), p.StateDir)
	require.Equal(t, filepath.Join(tmp, "custom-config"), p.ConfigDir)
}
```

- [ ] **Step 2.2: Run test to verify it fails**

```bash
go test ./internal/paths/...
```

Expected: FAIL — package does not exist yet.

- [ ] **Step 2.3: Implement `internal/paths/paths.go`**

```go
package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

type Paths struct {
	DataDir    string
	StateDir   string
	ConfigDir  string
	DBFile     string
	SocketFile string
	LogDir     string
	LogFile    string
}

func New() (*Paths, error) {
	dataDir, err := resolve("SPK_COCKPIT_DATA_DIR", "XDG_DATA_HOME", ".local/share", "spk-cockpit")
	if err != nil {
		return nil, err
	}
	stateDir, err := resolve("SPK_COCKPIT_STATE_DIR", "XDG_STATE_HOME", ".local/state", "spk-cockpit")
	if err != nil {
		return nil, err
	}
	configDir, err := resolve("SPK_COCKPIT_CONFIG_DIR", "XDG_CONFIG_HOME", ".config", "spk-cockpit")
	if err != nil {
		return nil, err
	}

	logDir := filepath.Join(stateDir, "log")
	for _, d := range []string{dataDir, stateDir, configDir, logDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, fmt.Errorf("mkdir %s: %w", d, err)
		}
	}

	return &Paths{
		DataDir:    dataDir,
		StateDir:   stateDir,
		ConfigDir:  configDir,
		LogDir:     logDir,
		DBFile:     filepath.Join(dataDir, "cockpit.db"),
		SocketFile: filepath.Join(stateDir, "cockpit.sock"),
		LogFile:    filepath.Join(logDir, "cockpit.log"),
	}, nil
}

func resolve(envVar, xdgVar, defaultRel, app string) (string, error) {
	if v := os.Getenv(envVar); v != "" {
		return v, nil
	}
	if v := os.Getenv(xdgVar); v != "" {
		return filepath.Join(v, app), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home: %w", err)
	}
	return filepath.Join(home, defaultRel, app), nil
}
```

- [ ] **Step 2.4: Add testify dependency**

```bash
go get github.com/stretchr/testify
go mod tidy
```

- [ ] **Step 2.5: Run test to verify it passes**

```bash
go test ./internal/paths/... -v
```

Expected: both tests PASS.

- [ ] **Step 2.6: Implement `internal/log/log.go`**

```go
package log

import (
	"io"
	"log/slog"
	"os"
)

func New(out io.Writer, level slog.Level) *slog.Logger {
	if out == nil {
		out = os.Stderr
	}
	return slog.New(slog.NewTextHandler(out, &slog.HandlerOptions{
		Level: level,
	}))
}

func ParseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
```

- [ ] **Step 2.7: Commit**

```bash
git add .
git commit -m "feat: add XDG paths and slog setup"
```

---

## Task 3: Implement Clock interface

**Files:**
- Create: `internal/clock/clock.go`
- Create: `internal/clock/fake.go`
- Create: `internal/clock/clock_test.go`

- [ ] **Step 3.1: Write the failing test**

```go
package clock

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRealClock_NowReturnsCurrentTime(t *testing.T) {
	c := Real()
	before := time.Now()
	got := c.Now()
	after := time.Now()
	require.True(t, !got.Before(before) && !got.After(after))
}

func TestFakeClock_NowReturnsSetValue(t *testing.T) {
	t0 := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	c := NewFake(t0)
	require.Equal(t, t0, c.Now())
}

func TestFakeClock_AdvanceMovesNow(t *testing.T) {
	t0 := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	c := NewFake(t0)
	c.Advance(5 * time.Minute)
	require.Equal(t, t0.Add(5*time.Minute), c.Now())
}
```

- [ ] **Step 3.2: Run test to verify it fails**

```bash
go test ./internal/clock/...
```

Expected: FAIL — package does not exist.

- [ ] **Step 3.3: Implement `internal/clock/clock.go`**

```go
package clock

import "time"

type Clock interface {
	Now() time.Time
}

type realClock struct{}

func Real() Clock { return realClock{} }

func (realClock) Now() time.Time { return time.Now().UTC() }
```

- [ ] **Step 3.4: Implement `internal/clock/fake.go`**

```go
package clock

import (
	"sync"
	"time"
)

type Fake struct {
	mu  sync.Mutex
	now time.Time
}

func NewFake(t time.Time) *Fake {
	return &Fake{now: t}
}

func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}

func (f *Fake) Set(t time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = t
}
```

- [ ] **Step 3.5: Run tests**

```bash
go test ./internal/clock/... -v
```

Expected: PASS.

- [ ] **Step 3.6: Commit**

```bash
git add .
git commit -m "feat: add Clock interface with real and fake impls"
```

---

## Task 4: Define API DTOs and event types

**Files:**
- Create: `internal/api/dto.go`
- Create: `internal/api/events.go`

- [ ] **Step 4.1: Write `internal/api/dto.go`**

```go
package api

type Priority int

const (
	PriorityLow    Priority = 0
	PriorityNormal Priority = 1
	PriorityHigh   Priority = 2
	PriorityUrgent Priority = 3
)

type TodoStatus string

const (
	StatusOpen       TodoStatus = "open"
	StatusInProgress TodoStatus = "in_progress"
	StatusDone       TodoStatus = "done"
	StatusCancelled  TodoStatus = "cancelled"
)

type Todo struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Notes     string     `json:"notes"`
	Priority  Priority   `json:"priority"`
	Status    TodoStatus `json:"status"`
	DueAt     *int64     `json:"dueAt,omitempty"`     // unix seconds, UTC
	Tags      []string   `json:"tags"`
	CreatedAt int64      `json:"createdAt"`
	UpdatedAt int64      `json:"updatedAt"`
	DoneAt    *int64     `json:"doneAt,omitempty"`
}

type Tag struct {
	Name      string `json:"name"`
	Color     string `json:"color"`
	CreatedAt int64  `json:"createdAt"`
}

type TodoEvent struct {
	ID        int64  `json:"id"`
	TodoID    string `json:"todoId"`
	Kind      string `json:"kind"`
	FromValue string `json:"fromValue,omitempty"`
	ToValue   string `json:"toValue,omitempty"`
	Payload   string `json:"payload,omitempty"`
	At        int64  `json:"at"`
}

type CreateTodoRequest struct {
	Title    string   `json:"title"`
	Notes    string   `json:"notes,omitempty"`
	Priority Priority `json:"priority"`
	DueAt    *int64   `json:"dueAt,omitempty"`
	Tags     []string `json:"tags,omitempty"`
}

type UpdateTodoRequest struct {
	Title    *string     `json:"title,omitempty"`
	Notes    *string     `json:"notes,omitempty"`
	Priority *Priority   `json:"priority,omitempty"`
	Status   *TodoStatus `json:"status,omitempty"`
	DueAt    *int64      `json:"dueAt,omitempty"`
	Tags     *[]string   `json:"tags,omitempty"`
}

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
```

- [ ] **Step 4.2: Write `internal/api/events.go`**

```go
package api

const (
	EventTodoCreated        = "todo.created"
	EventTodoUpdated        = "todo.updated"
	EventTodoStatusChanged  = "todo.status_changed"
	EventTodoDeleted        = "todo.deleted"
)

type Event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

type TodoCreatedData struct {
	Todo Todo `json:"todo"`
}

type TodoUpdatedData struct {
	Todo          Todo     `json:"todo"`
	ChangedFields []string `json:"changedFields"`
}

type TodoStatusChangedData struct {
	TodoID string     `json:"todoId"`
	From   TodoStatus `json:"from"`
	To     TodoStatus `json:"to"`
}

type TodoDeletedData struct {
	TodoID string `json:"todoId"`
}
```

- [ ] **Step 4.3: Verify compilation**

```bash
go build ./internal/api/...
```

Expected: no errors.

- [ ] **Step 4.4: Commit**

```bash
git add .
git commit -m "feat: define todo DTOs and event types"
```

---

## Task 5: Define repository interfaces

**Files:**
- Create: `internal/todo/repo.go`

- [ ] **Step 5.1: Write `internal/todo/repo.go`**

```go
package todo

import (
	"context"
	"errors"

	"github.com/spk/spk-cockpit/internal/api"
)

var (
	ErrNotFound      = errors.New("todo: not found")
	ErrTagNotFound   = errors.New("todo: tag not found")
	ErrInvalidStatus = errors.New("todo: invalid status transition")
)

type TodoFilter struct {
	Statuses    []api.TodoStatus
	Priorities  []api.Priority
	Tags        []string
	Search      string  // case-insensitive LIKE on title+notes
	IncludeDone bool    // if false: exclude status='done'|'cancelled'
	Limit       int     // 0 = no limit
}

type TodoRepo interface {
	Create(ctx context.Context, t api.Todo) error
	Get(ctx context.Context, id string) (api.Todo, error)
	Update(ctx context.Context, id string, mutate func(*api.Todo) error) (api.Todo, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, f TodoFilter) ([]api.Todo, error)
}

type TagRepo interface {
	Upsert(ctx context.Context, t api.Tag) error
	List(ctx context.Context) ([]api.Tag, error)
	Delete(ctx context.Context, name string) error
	SetTodoTags(ctx context.Context, todoID string, tags []string) error
	GetTodoTags(ctx context.Context, todoID string) ([]string, error)
}

type EventRepo interface {
	Append(ctx context.Context, e api.TodoEvent) error
	ListByTodo(ctx context.Context, todoID string, limit int) ([]api.TodoEvent, error)
	ListAll(ctx context.Context, sinceUnix int64, limit int) ([]api.TodoEvent, error)
}

type KvRepo interface {
	Get(ctx context.Context, key string) (string, bool, error)
	Set(ctx context.Context, key, value string) error
	Delete(ctx context.Context, key string) error
}
```

- [ ] **Step 5.2: Verify compilation**

```bash
go build ./internal/todo/...
```

Expected: no errors.

- [ ] **Step 5.3: Commit**

```bash
git add .
git commit -m "feat: define todo repository interfaces"
```

---

## Task 6: SQLite store skeleton + migration runner + initial migration

**Files:**
- Create: `internal/store/store.go`
- Create: `internal/store/migrate.go`
- Create: `internal/store/migrate_test.go`
- Create: `internal/store/migrations/0001_init.sql`
- Create: `internal/store/migrations/embed.go`

- [ ] **Step 6.1: Add SQLite driver dependency**

```bash
go get modernc.org/sqlite
go get github.com/oklog/ulid/v2
go mod tidy
```

- [ ] **Step 6.2: Write `internal/store/migrations/0001_init.sql`**

```sql
CREATE TABLE todos (
  id            TEXT PRIMARY KEY,
  title         TEXT NOT NULL,
  notes         TEXT NOT NULL DEFAULT '',
  priority      INTEGER NOT NULL,
  status        TEXT NOT NULL,
  due_at        INTEGER,
  created_at    INTEGER NOT NULL,
  updated_at    INTEGER NOT NULL,
  done_at       INTEGER,
  deleted_at    INTEGER
);
CREATE INDEX idx_todos_status_priority ON todos(status, priority DESC, due_at);
CREATE INDEX idx_todos_done_at ON todos(done_at) WHERE done_at IS NOT NULL;

CREATE TABLE tags (
  name       TEXT PRIMARY KEY,
  color      TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL
);

CREATE TABLE todo_tags (
  todo_id TEXT NOT NULL REFERENCES todos(id) ON DELETE CASCADE,
  tag     TEXT NOT NULL REFERENCES tags(name) ON DELETE CASCADE ON UPDATE CASCADE,
  PRIMARY KEY (todo_id, tag)
);
CREATE INDEX idx_todo_tags_tag ON todo_tags(tag);

CREATE TABLE todo_events (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  todo_id    TEXT NOT NULL,
  kind       TEXT NOT NULL,
  from_value TEXT,
  to_value   TEXT,
  payload    TEXT,
  at         INTEGER NOT NULL
);
CREATE INDEX idx_todo_events_todo_at ON todo_events(todo_id, at);

CREATE TABLE kv (
  k TEXT PRIMARY KEY,
  v TEXT NOT NULL
);

CREATE TABLE schema_migrations (
  version INTEGER PRIMARY KEY,
  applied_at INTEGER NOT NULL
);
```

- [ ] **Step 6.3: Write `internal/store/migrations/embed.go`**

```go
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
```

- [ ] **Step 6.4: Write `internal/store/store.go`**

```go
package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type Store struct {
	DB *sql.DB
}

func Open(dsn string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sql open: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite + WAL: serialize writes through one conn for simplicity
	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA busy_timeout = 5000",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("pragma %q: %w", p, err)
		}
	}
	return &Store{DB: db}, nil
}

func (s *Store) Close() error {
	return s.DB.Close()
}
```

- [ ] **Step 6.5: Write `internal/store/migrate.go`**

```go
package store

import (
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spk/spk-cockpit/internal/store/migrations"
)

func Migrate(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at INTEGER NOT NULL
	)`); err != nil {
		return fmt.Errorf("ensure migrations table: %w", err)
	}

	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}

	type m struct {
		version int
		name    string
	}
	var ms []m
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		// expect name like "0001_init.sql"
		parts := strings.SplitN(e.Name(), "_", 2)
		if len(parts) != 2 {
			return fmt.Errorf("bad migration filename %q", e.Name())
		}
		v, err := strconv.Atoi(parts[0])
		if err != nil {
			return fmt.Errorf("parse migration version %q: %w", parts[0], err)
		}
		ms = append(ms, m{version: v, name: e.Name()})
	}
	sort.Slice(ms, func(i, j int) bool { return ms[i].version < ms[j].version })

	for _, mig := range ms {
		var exists int
		if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, mig.version).Scan(&exists); err != nil {
			return fmt.Errorf("check migration %d: %w", mig.version, err)
		}
		if exists > 0 {
			continue
		}
		body, err := migrations.FS.ReadFile(mig.name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", mig.name, err)
		}
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration tx: %w", err)
		}
		if _, err := tx.Exec(string(body)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("exec migration %s: %w", mig.name, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES (?, ?)`, mig.version, time.Now().Unix()); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s: %w", mig.name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", mig.name, err)
		}
	}
	return nil
}
```

- [ ] **Step 6.6: Write `internal/store/migrate_test.go`**

```go
package store

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigrate_AppliesOnFreshDB(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "test.db") + "?_pragma=foreign_keys(1)"
	s, err := Open(dsn)
	require.NoError(t, err)
	defer s.Close()

	require.NoError(t, Migrate(s.DB))

	rows, err := s.DB.Query(`SELECT version FROM schema_migrations ORDER BY version`)
	require.NoError(t, err)
	defer rows.Close()

	var versions []int
	for rows.Next() {
		var v int
		require.NoError(t, rows.Scan(&v))
		versions = append(versions, v)
	}
	require.Equal(t, []int{1}, versions)

	for _, table := range []string{"todos", "tags", "todo_tags", "todo_events", "kv"} {
		var n int
		err := s.DB.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n)
		require.NoError(t, err, "table %s missing", table)
	}
}

func TestMigrate_IsIdempotent(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "test.db")
	s, err := Open(dsn)
	require.NoError(t, err)
	defer s.Close()
	require.NoError(t, Migrate(s.DB))
	require.NoError(t, Migrate(s.DB))
}
```

- [ ] **Step 6.7: Run tests**

```bash
go test ./internal/store/... -v
```

Expected: PASS for both tests.

- [ ] **Step 6.8: Commit**

```bash
git add .
git commit -m "feat: add SQLite store with migration runner"
```

---

## Task 7: Implement SQLite TodoRepo

**Files:**
- Create: `internal/store/todo_repo.go`

- [ ] **Step 7.1: Write `internal/store/todo_repo.go`**

```go
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/todo"
)

type TodoRepo struct {
	db *sql.DB
}

func NewTodoRepo(db *sql.DB) *TodoRepo { return &TodoRepo{db: db} }

func (r *TodoRepo) Create(ctx context.Context, t api.Todo) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO todos(id, title, notes, priority, status, due_at, created_at, updated_at, done_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, t.ID, t.Title, t.Notes, int(t.Priority), string(t.Status), t.DueAt, t.CreatedAt, t.UpdatedAt, t.DoneAt)
	if err != nil {
		return fmt.Errorf("insert todo: %w", err)
	}
	return nil
}

func (r *TodoRepo) Get(ctx context.Context, id string) (api.Todo, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, title, notes, priority, status, due_at, created_at, updated_at, done_at
		FROM todos WHERE id = ? AND deleted_at IS NULL
	`, id)
	t, err := scanTodo(row)
	if errors.Is(err, sql.ErrNoRows) {
		return api.Todo{}, todo.ErrNotFound
	}
	return t, err
}

func (r *TodoRepo) Update(ctx context.Context, id string, mutate func(*api.Todo) error) (api.Todo, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return api.Todo{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `
		SELECT id, title, notes, priority, status, due_at, created_at, updated_at, done_at
		FROM todos WHERE id = ? AND deleted_at IS NULL
	`, id)
	t, err := scanTodo(row)
	if errors.Is(err, sql.ErrNoRows) {
		return api.Todo{}, todo.ErrNotFound
	}
	if err != nil {
		return api.Todo{}, err
	}

	if err := mutate(&t); err != nil {
		return api.Todo{}, err
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE todos SET title=?, notes=?, priority=?, status=?, due_at=?, updated_at=?, done_at=?
		WHERE id=?
	`, t.Title, t.Notes, int(t.Priority), string(t.Status), t.DueAt, t.UpdatedAt, t.DoneAt, t.ID)
	if err != nil {
		return api.Todo{}, fmt.Errorf("update todo: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return api.Todo{}, fmt.Errorf("commit tx: %w", err)
	}
	return t, nil
}

func (r *TodoRepo) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `UPDATE todos SET deleted_at=strftime('%s','now') WHERE id=? AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("delete todo: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return todo.ErrNotFound
	}
	return nil
}

func (r *TodoRepo) List(ctx context.Context, f todo.TodoFilter) ([]api.Todo, error) {
	var (
		conds []string
		args  []any
	)
	conds = append(conds, "deleted_at IS NULL")
	if !f.IncludeDone {
		conds = append(conds, "status NOT IN ('done', 'cancelled')")
	}
	if len(f.Statuses) > 0 {
		ph := strings.Repeat("?,", len(f.Statuses))
		ph = ph[:len(ph)-1]
		conds = append(conds, "status IN ("+ph+")")
		for _, s := range f.Statuses {
			args = append(args, string(s))
		}
	}
	if len(f.Priorities) > 0 {
		ph := strings.Repeat("?,", len(f.Priorities))
		ph = ph[:len(ph)-1]
		conds = append(conds, "priority IN ("+ph+")")
		for _, p := range f.Priorities {
			args = append(args, int(p))
		}
	}
	if f.Search != "" {
		conds = append(conds, "(title LIKE ? OR notes LIKE ?)")
		s := "%" + f.Search + "%"
		args = append(args, s, s)
	}
	if len(f.Tags) > 0 {
		ph := strings.Repeat("?,", len(f.Tags))
		ph = ph[:len(ph)-1]
		conds = append(conds, "id IN (SELECT todo_id FROM todo_tags WHERE tag IN ("+ph+"))")
		for _, t := range f.Tags {
			args = append(args, t)
		}
	}
	q := `SELECT id, title, notes, priority, status, due_at, created_at, updated_at, done_at
		FROM todos WHERE ` + strings.Join(conds, " AND ") +
		` ORDER BY status='done' ASC, priority DESC, COALESCE(due_at, 9999999999) ASC, created_at DESC`
	if f.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", f.Limit)
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list todos: %w", err)
	}
	defer rows.Close()
	var out []api.Todo
	for rows.Next() {
		t, err := scanTodo(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

type scanner interface {
	Scan(...any) error
}

func scanTodo(s scanner) (api.Todo, error) {
	var t api.Todo
	var prio int
	var status string
	var dueAt, doneAt sql.NullInt64
	if err := s.Scan(&t.ID, &t.Title, &t.Notes, &prio, &status, &dueAt, &t.CreatedAt, &t.UpdatedAt, &doneAt); err != nil {
		return api.Todo{}, err
	}
	t.Priority = api.Priority(prio)
	t.Status = api.TodoStatus(status)
	if dueAt.Valid {
		v := dueAt.Int64
		t.DueAt = &v
	}
	if doneAt.Valid {
		v := doneAt.Int64
		t.DoneAt = &v
	}
	return t, nil
}
```

- [ ] **Step 7.2: Verify compilation**

```bash
go build ./internal/store/...
```

Expected: no errors.

- [ ] **Step 7.3: Commit**

```bash
git add .
git commit -m "feat: implement SQLite TodoRepo"
```

---

## Task 8: Implement SQLite TagRepo

**Files:**
- Create: `internal/store/tag_repo.go`

- [ ] **Step 8.1: Write `internal/store/tag_repo.go`**

```go
package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/spk/spk-cockpit/internal/api"
)

type TagRepo struct {
	db *sql.DB
}

func NewTagRepo(db *sql.DB) *TagRepo { return &TagRepo{db: db} }

func (r *TagRepo) Upsert(ctx context.Context, t api.Tag) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO tags(name, color, created_at) VALUES (?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET color=excluded.color
	`, t.Name, t.Color, t.CreatedAt)
	if err != nil {
		return fmt.Errorf("upsert tag: %w", err)
	}
	return nil
}

func (r *TagRepo) List(ctx context.Context) ([]api.Tag, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT name, color, created_at FROM tags ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	defer rows.Close()
	var out []api.Tag
	for rows.Next() {
		var t api.Tag
		if err := rows.Scan(&t.Name, &t.Color, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *TagRepo) Delete(ctx context.Context, name string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM tags WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("delete tag: %w", err)
	}
	return nil
}

func (r *TagRepo) SetTodoTags(ctx context.Context, todoID string, tags []string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM todo_tags WHERE todo_id = ?`, todoID); err != nil {
		return fmt.Errorf("clear todo_tags: %w", err)
	}
	for _, name := range tags {
		// auto-create tag if missing
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO tags(name, color, created_at) VALUES (?, '', strftime('%s','now'))
			 ON CONFLICT(name) DO NOTHING`, name); err != nil {
			return fmt.Errorf("ensure tag %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO todo_tags(todo_id, tag) VALUES (?, ?)`, todoID, name); err != nil {
			return fmt.Errorf("link todo_tag: %w", err)
		}
	}
	return tx.Commit()
}

func (r *TagRepo) GetTodoTags(ctx context.Context, todoID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT tag FROM todo_tags WHERE todo_id = ? ORDER BY tag`, todoID)
	if err != nil {
		return nil, fmt.Errorf("get todo_tags: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
```

- [ ] **Step 8.2: Verify compilation and commit**

```bash
go build ./internal/store/...
git add .
git commit -m "feat: implement SQLite TagRepo"
```

---

## Task 9: Implement SQLite EventRepo and KvRepo

**Files:**
- Create: `internal/store/event_repo.go`
- Create: `internal/store/kv_repo.go`

- [ ] **Step 9.1: Write `internal/store/event_repo.go`**

```go
package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/spk/spk-cockpit/internal/api"
)

type EventRepo struct {
	db *sql.DB
}

func NewEventRepo(db *sql.DB) *EventRepo { return &EventRepo{db: db} }

func (r *EventRepo) Append(ctx context.Context, e api.TodoEvent) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO todo_events(todo_id, kind, from_value, to_value, payload, at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, e.TodoID, e.Kind, nullStr(e.FromValue), nullStr(e.ToValue), nullStr(e.Payload), e.At)
	if err != nil {
		return fmt.Errorf("insert todo_event: %w", err)
	}
	return nil
}

func (r *EventRepo) ListByTodo(ctx context.Context, todoID string, limit int) ([]api.TodoEvent, error) {
	q := `SELECT id, todo_id, kind, COALESCE(from_value,''), COALESCE(to_value,''), COALESCE(payload,''), at
		FROM todo_events WHERE todo_id = ? ORDER BY at DESC, id DESC`
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := r.db.QueryContext(ctx, q, todoID)
	return scanEvents(rows, err)
}

func (r *EventRepo) ListAll(ctx context.Context, sinceUnix int64, limit int) ([]api.TodoEvent, error) {
	q := `SELECT id, todo_id, kind, COALESCE(from_value,''), COALESCE(to_value,''), COALESCE(payload,''), at
		FROM todo_events WHERE at >= ? ORDER BY at DESC, id DESC`
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := r.db.QueryContext(ctx, q, sinceUnix)
	return scanEvents(rows, err)
}

func scanEvents(rows *sql.Rows, err error) ([]api.TodoEvent, error) {
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()
	var out []api.TodoEvent
	for rows.Next() {
		var e api.TodoEvent
		if err := rows.Scan(&e.ID, &e.TodoID, &e.Kind, &e.FromValue, &e.ToValue, &e.Payload, &e.At); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
```

- [ ] **Step 9.2: Write `internal/store/kv_repo.go`**

```go
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type KvRepo struct {
	db *sql.DB
}

func NewKvRepo(db *sql.DB) *KvRepo { return &KvRepo{db: db} }

func (r *KvRepo) Get(ctx context.Context, key string) (string, bool, error) {
	var v string
	err := r.db.QueryRowContext(ctx, `SELECT v FROM kv WHERE k = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("kv get: %w", err)
	}
	return v, true, nil
}

func (r *KvRepo) Set(ctx context.Context, key, value string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO kv(k, v) VALUES (?, ?)
		ON CONFLICT(k) DO UPDATE SET v=excluded.v
	`, key, value)
	if err != nil {
		return fmt.Errorf("kv set: %w", err)
	}
	return nil
}

func (r *KvRepo) Delete(ctx context.Context, key string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM kv WHERE k = ?`, key)
	if err != nil {
		return fmt.Errorf("kv delete: %w", err)
	}
	return nil
}
```

- [ ] **Step 9.3: Verify compilation and commit**

```bash
go build ./internal/store/...
git add .
git commit -m "feat: implement SQLite EventRepo and KvRepo"
```

---

## Task 10: Implement fake repos and conformance tests

**Files:**
- Create: `internal/todo/fakerepo/todo_repo.go`
- Create: `internal/todo/fakerepo/tag_repo.go`
- Create: `internal/todo/fakerepo/event_repo.go`
- Create: `internal/store/conformance_test.go`

- [ ] **Step 10.1: Write `internal/todo/fakerepo/todo_repo.go`**

```go
package fakerepo

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/todo"
)

type Todo struct {
	mu     sync.Mutex
	byID   map[string]api.Todo
	delAt  map[string]int64
}

func NewTodo() *Todo {
	return &Todo{byID: map[string]api.Todo{}, delAt: map[string]int64{}}
}

func (r *Todo) Create(_ context.Context, t api.Todo) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[t.ID] = t
	return nil
}

func (r *Todo) Get(_ context.Context, id string) (api.Todo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, deleted := r.delAt[id]; deleted {
		return api.Todo{}, todo.ErrNotFound
	}
	t, ok := r.byID[id]
	if !ok {
		return api.Todo{}, todo.ErrNotFound
	}
	return t, nil
}

func (r *Todo) Update(_ context.Context, id string, mutate func(*api.Todo) error) (api.Todo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, deleted := r.delAt[id]; deleted {
		return api.Todo{}, todo.ErrNotFound
	}
	t, ok := r.byID[id]
	if !ok {
		return api.Todo{}, todo.ErrNotFound
	}
	if err := mutate(&t); err != nil {
		return api.Todo{}, err
	}
	r.byID[id] = t
	return t, nil
}

func (r *Todo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[id]; !ok {
		return todo.ErrNotFound
	}
	if _, already := r.delAt[id]; already {
		return todo.ErrNotFound
	}
	r.delAt[id] = 1
	return nil
}

func (r *Todo) List(_ context.Context, f todo.TodoFilter) ([]api.Todo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []api.Todo
	for id, t := range r.byID {
		if _, deleted := r.delAt[id]; deleted {
			continue
		}
		if !f.IncludeDone && (t.Status == api.StatusDone || t.Status == api.StatusCancelled) {
			continue
		}
		if len(f.Statuses) > 0 && !contains(f.Statuses, t.Status) {
			continue
		}
		if len(f.Priorities) > 0 && !containsP(f.Priorities, t.Priority) {
			continue
		}
		if f.Search != "" {
			s := strings.ToLower(f.Search)
			if !strings.Contains(strings.ToLower(t.Title), s) && !strings.Contains(strings.ToLower(t.Notes), s) {
				continue
			}
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority > out[j].Priority
		}
		return out[i].CreatedAt > out[j].CreatedAt
	})
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

func contains(xs []api.TodoStatus, x api.TodoStatus) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func containsP(xs []api.Priority, x api.Priority) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
```

- [ ] **Step 10.2: Write `internal/todo/fakerepo/tag_repo.go`**

```go
package fakerepo

import (
	"context"
	"sort"
	"sync"

	"github.com/spk/spk-cockpit/internal/api"
)

type Tag struct {
	mu       sync.Mutex
	tags     map[string]api.Tag
	todoTags map[string]map[string]struct{}
}

func NewTag() *Tag {
	return &Tag{tags: map[string]api.Tag{}, todoTags: map[string]map[string]struct{}{}}
}

func (r *Tag) Upsert(_ context.Context, t api.Tag) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tags[t.Name] = t
	return nil
}

func (r *Tag) List(_ context.Context) ([]api.Tag, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]api.Tag, 0, len(r.tags))
	for _, t := range r.tags {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (r *Tag) Delete(_ context.Context, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tags, name)
	for _, m := range r.todoTags {
		delete(m, name)
	}
	return nil
}

func (r *Tag) SetTodoTags(_ context.Context, todoID string, tags []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := map[string]struct{}{}
	for _, name := range tags {
		if _, ok := r.tags[name]; !ok {
			r.tags[name] = api.Tag{Name: name, CreatedAt: 0}
		}
		m[name] = struct{}{}
	}
	r.todoTags[todoID] = m
	return nil
}

func (r *Tag) GetTodoTags(_ context.Context, todoID string) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := r.todoTags[todoID]
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}
```

- [ ] **Step 10.3: Write `internal/todo/fakerepo/event_repo.go`**

```go
package fakerepo

import (
	"context"
	"sort"
	"sync"

	"github.com/spk/spk-cockpit/internal/api"
)

type Event struct {
	mu     sync.Mutex
	nextID int64
	rows   []api.TodoEvent
}

func NewEvent() *Event { return &Event{} }

func (r *Event) Append(_ context.Context, e api.TodoEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	e.ID = r.nextID
	r.rows = append(r.rows, e)
	return nil
}

func (r *Event) ListByTodo(_ context.Context, todoID string, limit int) ([]api.TodoEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []api.TodoEvent
	for _, e := range r.rows {
		if e.TodoID == todoID {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At > out[j].At })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *Event) ListAll(_ context.Context, sinceUnix int64, limit int) ([]api.TodoEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []api.TodoEvent
	for _, e := range r.rows {
		if e.At >= sinceUnix {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At > out[j].At })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
```

- [ ] **Step 10.4: Write `internal/store/conformance_test.go`**

```go
package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/todo"
	"github.com/spk/spk-cockpit/internal/todo/fakerepo"
)

type todoRepoCase struct {
	name string
	new  func(t *testing.T) todo.TodoRepo
}

func todoRepoCases(t *testing.T) []todoRepoCase {
	return []todoRepoCase{
		{
			name: "fake",
			new:  func(t *testing.T) todo.TodoRepo { return fakerepo.NewTodo() },
		},
		{
			name: "sqlite",
			new: func(t *testing.T) todo.TodoRepo {
				dsn := "file:" + filepath.Join(t.TempDir(), "t.db")
				s, err := Open(dsn)
				require.NoError(t, err)
				t.Cleanup(func() { _ = s.Close() })
				require.NoError(t, Migrate(s.DB))
				return NewTodoRepo(s.DB)
			},
		},
	}
}

func TestTodoRepo_Conformance(t *testing.T) {
	for _, c := range todoRepoCases(t) {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			r := c.new(t)

			td := api.Todo{
				ID: "01H000000000000000000A0001", Title: "Hello",
				Priority: api.PriorityNormal, Status: api.StatusOpen,
				CreatedAt: 100, UpdatedAt: 100,
			}
			require.NoError(t, r.Create(ctx, td))

			got, err := r.Get(ctx, td.ID)
			require.NoError(t, err)
			require.Equal(t, td.Title, got.Title)

			_, err = r.Update(ctx, td.ID, func(x *api.Todo) error {
				x.Title = "Updated"
				x.UpdatedAt = 200
				return nil
			})
			require.NoError(t, err)
			got, err = r.Get(ctx, td.ID)
			require.NoError(t, err)
			require.Equal(t, "Updated", got.Title)

			list, err := r.List(ctx, todo.TodoFilter{})
			require.NoError(t, err)
			require.Len(t, list, 1)

			require.NoError(t, r.Delete(ctx, td.ID))
			_, err = r.Get(ctx, td.ID)
			require.ErrorIs(t, err, todo.ErrNotFound)
		})
	}
}
```

- [ ] **Step 10.5: Run conformance tests**

```bash
go test ./internal/store/... -v
```

Expected: PASS for both fake and sqlite implementations.

- [ ] **Step 10.6: Commit**

```bash
git add .
git commit -m "feat: add fake repos and TodoRepo conformance tests"
```

---

## Task 11: Implement Todo domain service

**Files:**
- Create: `internal/todo/service.go`
- Create: `internal/todo/service_test.go`

The service is the only place that mutates todos. It enforces invariants, emits audit events, and (later) publishes domain events to the bus. In phase 1, the bus is injected as an interface but may be nil — service handles nil safely.

- [ ] **Step 11.1: Write the failing test for `Create`**

```go
package todo_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/clock"
	"github.com/spk/spk-cockpit/internal/todo"
	"github.com/spk/spk-cockpit/internal/todo/fakerepo"
)

func newSvc(t *testing.T, now time.Time) (*todo.Service, *fakerepo.Todo, *fakerepo.Tag, *fakerepo.Event) {
	tr, gr, er := fakerepo.NewTodo(), fakerepo.NewTag(), fakerepo.NewEvent()
	c := clock.NewFake(now)
	return todo.NewService(tr, gr, er, c, nil), tr, gr, er
}

func TestService_Create_AssignsIDAndTimestamps(t *testing.T) {
	now := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	s, _, _, er := newSvc(t, now)

	got, err := s.Create(context.Background(), api.CreateTodoRequest{
		Title:    "Buy milk",
		Priority: api.PriorityNormal,
	})
	require.NoError(t, err)
	require.NotEmpty(t, got.ID)
	require.Equal(t, "Buy milk", got.Title)
	require.Equal(t, api.StatusOpen, got.Status)
	require.Equal(t, now.Unix(), got.CreatedAt)
	require.Equal(t, now.Unix(), got.UpdatedAt)
	require.Nil(t, got.DoneAt)

	events, err := er.ListByTodo(context.Background(), got.ID, 0)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, "created", events[0].Kind)
}

func TestService_Create_RejectsEmptyTitle(t *testing.T) {
	s, _, _, _ := newSvc(t, time.Now())
	_, err := s.Create(context.Background(), api.CreateTodoRequest{Title: "  "})
	require.Error(t, err)
}
```

- [ ] **Step 11.2: Run test to verify it fails**

```bash
go test ./internal/todo/... -v -run TestService
```

Expected: FAIL — `todo.NewService` undefined.

- [ ] **Step 11.3: Write `internal/todo/service.go` (Create + helpers)**

```go
package todo

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/oklog/ulid/v2"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/clock"
)

type EventPublisher interface {
	Publish(api.Event)
}

type Service struct {
	todos  TodoRepo
	tags   TagRepo
	events EventRepo
	clock  clock.Clock
	bus    EventPublisher
}

func NewService(t TodoRepo, g TagRepo, e EventRepo, c clock.Clock, bus EventPublisher) *Service {
	return &Service{todos: t, tags: g, events: e, clock: c, bus: bus}
}

func (s *Service) publish(t string, data any) {
	if s.bus == nil {
		return
	}
	s.bus.Publish(api.Event{Type: t, Data: data})
}

func (s *Service) Create(ctx context.Context, req api.CreateTodoRequest) (api.Todo, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return api.Todo{}, errors.New("title is required")
	}
	now := s.clock.Now().Unix()
	t := api.Todo{
		ID:        newULID(),
		Title:     title,
		Notes:     req.Notes,
		Priority:  req.Priority,
		Status:    api.StatusOpen,
		DueAt:     req.DueAt,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.todos.Create(ctx, t); err != nil {
		return api.Todo{}, fmt.Errorf("create todo: %w", err)
	}
	if len(req.Tags) > 0 {
		if err := s.tags.SetTodoTags(ctx, t.ID, req.Tags); err != nil {
			return api.Todo{}, fmt.Errorf("set tags: %w", err)
		}
	}
	if err := s.events.Append(ctx, api.TodoEvent{TodoID: t.ID, Kind: "created", At: now}); err != nil {
		return api.Todo{}, fmt.Errorf("audit: %w", err)
	}
	t.Tags = req.Tags
	s.publish(api.EventTodoCreated, api.TodoCreatedData{Todo: t})
	return t, nil
}

func newULID() string {
	return ulid.MustNew(ulid.Now(), rand.Reader).String()
}

func _unused() { _ = json.Marshal } // keep import — used in later steps
```

- [ ] **Step 11.4: Run test to verify Create passes**

```bash
go test ./internal/todo/... -v -run TestService_Create
```

Expected: PASS.

- [ ] **Step 11.5: Write tests for Get / Update / Delete / List**

Append to `internal/todo/service_test.go`:

```go
func TestService_Get_LoadsTags(t *testing.T) {
	now := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	s, _, _, _ := newSvc(t, now)
	got, err := s.Create(context.Background(), api.CreateTodoRequest{Title: "X", Tags: []string{"backend", "urgent"}})
	require.NoError(t, err)

	loaded, err := s.Get(context.Background(), got.ID)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"backend", "urgent"}, loaded.Tags)
}

func TestService_Update_StatusChangeEmitsEventAndDoneAt(t *testing.T) {
	t0 := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	s, _, _, er := newSvc(t, t0)
	got, err := s.Create(context.Background(), api.CreateTodoRequest{Title: "X"})
	require.NoError(t, err)

	done := api.StatusDone
	updated, err := s.Update(context.Background(), got.ID, api.UpdateTodoRequest{Status: &done})
	require.NoError(t, err)
	require.Equal(t, api.StatusDone, updated.Status)
	require.NotNil(t, updated.DoneAt)

	events, err := er.ListByTodo(context.Background(), got.ID, 0)
	require.NoError(t, err)
	kinds := []string{}
	for _, e := range events {
		kinds = append(kinds, e.Kind)
	}
	require.Contains(t, kinds, "status_changed")
}

func TestService_Update_PriorityChangeEmitsEvent(t *testing.T) {
	s, _, _, er := newSvc(t, time.Now())
	got, err := s.Create(context.Background(), api.CreateTodoRequest{Title: "X", Priority: api.PriorityNormal})
	require.NoError(t, err)

	urgent := api.PriorityUrgent
	_, err = s.Update(context.Background(), got.ID, api.UpdateTodoRequest{Priority: &urgent})
	require.NoError(t, err)

	events, err := er.ListByTodo(context.Background(), got.ID, 0)
	require.NoError(t, err)
	kinds := []string{}
	for _, e := range events {
		kinds = append(kinds, e.Kind)
	}
	require.Contains(t, kinds, "priority_changed")
}

func TestService_Delete_AppendsEventAndHidesFromGet(t *testing.T) {
	s, _, _, er := newSvc(t, time.Now())
	got, err := s.Create(context.Background(), api.CreateTodoRequest{Title: "X"})
	require.NoError(t, err)

	require.NoError(t, s.Delete(context.Background(), got.ID))

	_, err = s.Get(context.Background(), got.ID)
	require.ErrorIs(t, err, todo.ErrNotFound)

	events, err := er.ListByTodo(context.Background(), got.ID, 0)
	require.NoError(t, err)
	last := events[0]
	require.Equal(t, "deleted", last.Kind)
}

func TestService_List_FiltersAndExcludesDoneByDefault(t *testing.T) {
	s, _, _, _ := newSvc(t, time.Now())
	a, _ := s.Create(context.Background(), api.CreateTodoRequest{Title: "Open"})
	b, _ := s.Create(context.Background(), api.CreateTodoRequest{Title: "Done"})
	done := api.StatusDone
	_, _ = s.Update(context.Background(), b.ID, api.UpdateTodoRequest{Status: &done})

	got, err := s.List(context.Background(), todo.TodoFilter{})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, a.ID, got[0].ID)

	got, err = s.List(context.Background(), todo.TodoFilter{IncludeDone: true})
	require.NoError(t, err)
	require.Len(t, got, 2)
}
```

- [ ] **Step 11.6: Implement Get / Update / Delete / List in service**

Append to `internal/todo/service.go`:

```go
func (s *Service) Get(ctx context.Context, id string) (api.Todo, error) {
	t, err := s.todos.Get(ctx, id)
	if err != nil {
		return api.Todo{}, err
	}
	tags, err := s.tags.GetTodoTags(ctx, id)
	if err != nil {
		return api.Todo{}, fmt.Errorf("get tags: %w", err)
	}
	t.Tags = tags
	return t, nil
}

func (s *Service) Update(ctx context.Context, id string, req api.UpdateTodoRequest) (api.Todo, error) {
	now := s.clock.Now().Unix()
	var changed []string
	var (
		statusFrom, statusTo api.TodoStatus
		statusChanged        bool
		prioFrom, prioTo     api.Priority
		prioChanged          bool
	)

	updated, err := s.todos.Update(ctx, id, func(t *api.Todo) error {
		if req.Title != nil {
			if strings.TrimSpace(*req.Title) == "" {
				return errors.New("title is required")
			}
			if t.Title != *req.Title {
				t.Title = *req.Title
				changed = append(changed, "title")
			}
		}
		if req.Notes != nil && t.Notes != *req.Notes {
			t.Notes = *req.Notes
			changed = append(changed, "notes")
		}
		if req.Priority != nil && t.Priority != *req.Priority {
			prioFrom, prioTo = t.Priority, *req.Priority
			t.Priority = *req.Priority
			prioChanged = true
			changed = append(changed, "priority")
		}
		if req.Status != nil && t.Status != *req.Status {
			statusFrom, statusTo = t.Status, *req.Status
			t.Status = *req.Status
			statusChanged = true
			changed = append(changed, "status")
			if t.Status == api.StatusDone {
				now := now
				t.DoneAt = &now
			} else {
				t.DoneAt = nil
			}
		}
		if req.DueAt != nil {
			t.DueAt = req.DueAt
			changed = append(changed, "dueAt")
		}
		t.UpdatedAt = now
		return nil
	})
	if err != nil {
		return api.Todo{}, err
	}

	if req.Tags != nil {
		if err := s.tags.SetTodoTags(ctx, id, *req.Tags); err != nil {
			return api.Todo{}, fmt.Errorf("set tags: %w", err)
		}
		changed = append(changed, "tags")
		updated.Tags = *req.Tags
	} else {
		tags, err := s.tags.GetTodoTags(ctx, id)
		if err != nil {
			return api.Todo{}, fmt.Errorf("get tags: %w", err)
		}
		updated.Tags = tags
	}

	if statusChanged {
		_ = s.events.Append(ctx, api.TodoEvent{TodoID: id, Kind: "status_changed", FromValue: string(statusFrom), ToValue: string(statusTo), At: now})
		s.publish(api.EventTodoStatusChanged, api.TodoStatusChangedData{TodoID: id, From: statusFrom, To: statusTo})
	}
	if prioChanged {
		_ = s.events.Append(ctx, api.TodoEvent{TodoID: id, Kind: "priority_changed", FromValue: priorityStr(prioFrom), ToValue: priorityStr(prioTo), At: now})
	}
	if len(changed) > 0 {
		s.publish(api.EventTodoUpdated, api.TodoUpdatedData{Todo: updated, ChangedFields: changed})
		_ = s.events.Append(ctx, api.TodoEvent{TodoID: id, Kind: "edited", Payload: marshalChanged(changed), At: now})
	}
	return updated, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	now := s.clock.Now().Unix()
	if err := s.todos.Delete(ctx, id); err != nil {
		return err
	}
	_ = s.events.Append(ctx, api.TodoEvent{TodoID: id, Kind: "deleted", At: now})
	s.publish(api.EventTodoDeleted, api.TodoDeletedData{TodoID: id})
	return nil
}

func (s *Service) List(ctx context.Context, f TodoFilter) ([]api.Todo, error) {
	list, err := s.todos.List(ctx, f)
	if err != nil {
		return nil, err
	}
	for i := range list {
		tags, err := s.tags.GetTodoTags(ctx, list[i].ID)
		if err != nil {
			return nil, fmt.Errorf("get tags for %s: %w", list[i].ID, err)
		}
		list[i].Tags = tags
	}
	return list, nil
}

func (s *Service) History(ctx context.Context, id string, limit int) ([]api.TodoEvent, error) {
	return s.events.ListByTodo(ctx, id, limit)
}

func priorityStr(p api.Priority) string {
	switch p {
	case api.PriorityLow:
		return "low"
	case api.PriorityHigh:
		return "high"
	case api.PriorityUrgent:
		return "urgent"
	default:
		return "normal"
	}
}

func marshalChanged(changed []string) string {
	b, _ := json.Marshal(map[string]any{"changedFields": changed})
	return string(b)
}
```

Replace the `_unused` shim in step 11.3 with this real usage of `json` (delete the `_unused` function).

- [ ] **Step 11.7: Run all service tests**

```bash
go test ./internal/todo/... -v
```

Expected: all PASS.

- [ ] **Step 11.8: Commit**

```bash
git add .
git commit -m "feat: implement Todo domain service with audit events"
```

---

## Task 12: Implement EventBus

**Files:**
- Create: `internal/eventbus/bus.go`
- Create: `internal/eventbus/bus_test.go`

- [ ] **Step 12.1: Write the failing test**

```go
package eventbus_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/eventbus"
)

func TestBus_PublishesToAllSubscribers(t *testing.T) {
	b := eventbus.New(8)
	defer b.Close()

	ch1 := b.Subscribe(8)
	ch2 := b.Subscribe(8)

	b.Publish(api.Event{Type: "x", Data: 1})

	for _, ch := range []<-chan api.Event{ch1, ch2} {
		select {
		case e := <-ch:
			require.Equal(t, "x", e.Type)
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for event")
		}
	}
}

func TestBus_DropsForSlowSubscriber(t *testing.T) {
	b := eventbus.New(8)
	defer b.Close()

	ch := b.Subscribe(1) // tiny buffer
	for i := 0; i < 100; i++ {
		b.Publish(api.Event{Type: "x"})
	}
	// We should still be able to read at least one event without deadlock
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("bus blocked")
	}
}

func TestBus_UnsubscribeStopsDelivery(t *testing.T) {
	b := eventbus.New(8)
	defer b.Close()

	ch := b.Subscribe(8)
	b.Unsubscribe(ch)
	b.Publish(api.Event{Type: "x"})

	var got int32
	go func() {
		for range ch {
			atomic.AddInt32(&got, 1)
		}
	}()
	time.Sleep(50 * time.Millisecond)
	require.Equal(t, int32(0), atomic.LoadInt32(&got))
}
```

- [ ] **Step 12.2: Run test to verify it fails**

```bash
go test ./internal/eventbus/...
```

Expected: FAIL — package missing.

- [ ] **Step 12.3: Implement `internal/eventbus/bus.go`**

```go
package eventbus

import (
	"sync"

	"github.com/spk/spk-cockpit/internal/api"
)

type Bus struct {
	mu     sync.RWMutex
	subs   map[chan api.Event]struct{}
	closed bool
}

func New(_ int) *Bus {
	return &Bus{subs: map[chan api.Event]struct{}{}}
}

func (b *Bus) Subscribe(buf int) chan api.Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan api.Event, buf)
	if b.closed {
		close(ch)
		return ch
	}
	b.subs[ch] = struct{}{}
	return ch
}

func (b *Bus) Unsubscribe(ch chan api.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.subs[ch]; !ok {
		return
	}
	delete(b.subs, ch)
	close(ch)
}

func (b *Bus) Publish(e api.Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return
	}
	for ch := range b.subs {
		select {
		case ch <- e:
		default:
			// drop on slow consumer
		}
	}
}

func (b *Bus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for ch := range b.subs {
		close(ch)
	}
	b.subs = nil
}
```

`Bus.Subscribe` returns `chan api.Event` (not `<-chan`) so callers can pass it back to `Unsubscribe`. The test consumes via the directional alias only.

- [ ] **Step 12.4: Run tests**

```bash
go test ./internal/eventbus/... -v
```

Expected: PASS.

- [ ] **Step 12.5: Commit**

```bash
git add .
git commit -m "feat: add in-memory event bus"
```

---

## Task 13: HTTP server skeleton on UDS with health endpoint

**Files:**
- Create: `internal/server/server.go`
- Create: `internal/server/middleware.go`
- Create: `internal/server/health_handler.go`
- Create: `internal/server/routes.go`
- Create: `internal/server/server_test.go`

- [ ] **Step 13.1: Write the failing health-endpoint test**

```go
package server_test

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/spk/spk-cockpit/internal/server"
)

func TestServer_HealthEndpoint(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "test.sock")
	srv, err := server.New(server.Config{SocketPath: sock})
	require.NoError(t, err)
	go func() { _ = srv.Serve() }()
	defer srv.Stop(context.Background())
	waitForSocket(t, sock)

	c := &http.Client{Transport: &http.Transport{
		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", sock)
		},
	}}
	resp, err := c.Get("http://unix/api/health")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode)

	var body map[string]any
	b, _ := io.ReadAll(resp.Body)
	require.NoError(t, json.Unmarshal(b, &body))
	require.Equal(t, "ok", body["status"])
}

func waitForSocket(t *testing.T, path string) {
	for i := 0; i < 50; i++ {
		if _, err := net.Dial("unix", path); err == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("socket not ready")
}
```

- [ ] **Step 13.2: Run test to verify it fails**

```bash
go test ./internal/server/... -v
```

Expected: FAIL — package missing.

- [ ] **Step 13.3: Implement `internal/server/server.go`**

```go
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
)

type Config struct {
	SocketPath string
	Logger     *slog.Logger
}

type Server struct {
	cfg      Config
	listener net.Listener
	httpSrv  *http.Server
	logger   *slog.Logger
	deps     *Deps
}

type Deps struct {
	// Filled in by registerRoutes consumers in later tasks (TodoService, TagRepo, EventBus).
	// In phase 1 task 13, the Deps struct exists but is empty.
}

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
	s := &Server{cfg: cfg, listener: ln, logger: logger, deps: &Deps{}}
	mux := http.NewServeMux()
	registerRoutes(mux, s.deps)
	s.httpSrv = &http.Server{
		Handler:           recover(logger, requestLog(logger, mux)),
		ReadHeaderTimeout: 5 * time.Second,
	}
	return s, nil
}

func (s *Server) Serve() error {
	err := s.httpSrv.Serve(s.listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) Stop(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return s.httpSrv.Shutdown(shutdownCtx)
}

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
```

- [ ] **Step 13.4: Implement `internal/server/middleware.go`**

```go
package server

import (
	"log/slog"
	"net/http"
	"time"
)

func recover(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rv := recoverPanic(); rv != nil {
				log.Error("panic in handler", "path", r.URL.Path, "value", rv)
				writeError(w, http.StatusInternalServerError, "internal", "internal error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func recoverPanic() (out any) {
	if r := recoverImpl(); r != nil {
		out = r
	}
	return
}

func recoverImpl() any {
	if r := recover(); r != nil {
		return r
	}
	return nil
}

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
```

The two `recover*` helpers separate the language built-in `recover()` (used inside a deferred call) from our middleware function, which is also named `recover` for readability. The double indirection is a workaround for the name shadowing — adjust to the simpler form below if you prefer:

```go
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
```

If you take the second form, also rename the call site in `New()` from `recover(...)` to `recoverMW(...)` and delete the helpers. Do whichever is cleaner — the test does not depend on the choice.

- [ ] **Step 13.5: Implement `internal/server/health_handler.go`**

```go
package server

import (
	"encoding/json"
	"net/http"

	"github.com/spk/spk-cockpit/internal/api"
)

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, api.ErrorResponse{Error: api.ErrorBody{Code: code, Message: msg}})
}
```

- [ ] **Step 13.6: Implement `internal/server/routes.go`**

```go
package server

import "net/http"

func registerRoutes(mux *http.ServeMux, _ *Deps) {
	mux.HandleFunc("GET /api/health", handleHealth)
}
```

- [ ] **Step 13.7: Run health test**

```bash
go test ./internal/server/... -v
```

Expected: PASS.

- [ ] **Step 13.8: Commit**

```bash
git add .
git commit -m "feat: add UDS HTTP server with health endpoint"
```

---

## Task 14: Wire Todo service into server with REST handlers

**Files:**
- Modify: `internal/server/server.go` (extend `Deps`)
- Modify: `internal/server/routes.go`
- Create: `internal/server/todo_handler.go`
- Create: `internal/server/todo_handler_test.go`

- [ ] **Step 14.1: Extend `Deps` struct**

Edit `internal/server/server.go` — replace the `Deps` definition with:

```go
type Deps struct {
	Todos *todo.Service
	Tags  todo.TagRepo
	Bus   *eventbus.Bus
}
```

Add imports:

```go
import (
	"github.com/spk/spk-cockpit/internal/eventbus"
	"github.com/spk/spk-cockpit/internal/todo"
)
```

- [ ] **Step 14.2: Write `internal/server/todo_handler.go`**

```go
package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/todo"
)

func handleListTodos(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f := todo.TodoFilter{}
		q := r.URL.Query()
		if v := q.Get("includeDone"); v == "1" || v == "true" {
			f.IncludeDone = true
		}
		if v := q.Get("search"); v != "" {
			f.Search = v
		}
		for _, s := range q["status"] {
			f.Statuses = append(f.Statuses, api.TodoStatus(s))
		}
		for _, p := range q["priority"] {
			n, err := strconv.Atoi(p)
			if err == nil {
				f.Priorities = append(f.Priorities, api.Priority(n))
			}
		}
		for _, t := range q["tag"] {
			if t != "" {
				f.Tags = append(f.Tags, t)
			}
		}
		if v := q.Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				f.Limit = n
			}
		}
		list, err := d.Todos.List(r.Context(), f)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "todo.list_failed", err.Error())
			return
		}
		if list == nil {
			list = []api.Todo{}
		}
		writeJSON(w, http.StatusOK, list)
	}
}

func handleCreateTodo(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req api.CreateTodoRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		t, err := d.Todos.Create(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, "todo.create_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, t)
	}
}

func handleGetTodo(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		t, err := d.Todos.Get(r.Context(), id)
		if errors.Is(err, todo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "todo.not_found", "todo not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "todo.get_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, t)
	}
}

func handleUpdateTodo(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var req api.UpdateTodoRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		t, err := d.Todos.Update(r.Context(), id, req)
		if errors.Is(err, todo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "todo.not_found", "todo not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "todo.update_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, t)
	}
}

func handleDeleteTodo(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		err := d.Todos.Delete(r.Context(), id)
		if errors.Is(err, todo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "todo.not_found", "todo not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "todo.delete_failed", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleHistoryTodo(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		limit := 100
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				limit = n
			}
		}
		events, err := d.Todos.History(r.Context(), id, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "todo.history_failed", err.Error())
			return
		}
		if events == nil {
			events = []api.TodoEvent{}
		}
		writeJSON(w, http.StatusOK, events)
	}
}

func handleListTags(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tags, err := d.Tags.List(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "tag.list_failed", err.Error())
			return
		}
		if tags == nil {
			tags = []api.Tag{}
		}
		writeJSON(w, http.StatusOK, tags)
	}
}

// helper for tests
func _ignore() { _ = strings.TrimSpace }
```

(`_ignore` is a placeholder until later tasks reference `strings`; if the linter complains, delete the dummy and the import.)

- [ ] **Step 14.3: Update `internal/server/routes.go`**

```go
package server

import "net/http"

func registerRoutes(mux *http.ServeMux, d *Deps) {
	mux.HandleFunc("GET /api/health", handleHealth)

	mux.HandleFunc("GET /api/todos", handleListTodos(d))
	mux.HandleFunc("POST /api/todos", handleCreateTodo(d))
	mux.HandleFunc("GET /api/todos/{id}", handleGetTodo(d))
	mux.HandleFunc("PATCH /api/todos/{id}", handleUpdateTodo(d))
	mux.HandleFunc("DELETE /api/todos/{id}", handleDeleteTodo(d))
	mux.HandleFunc("GET /api/todos/{id}/history", handleHistoryTodo(d))

	mux.HandleFunc("GET /api/tags", handleListTags(d))
}
```

- [ ] **Step 14.4: Modify `New()` to allow setting Deps before listening**

Replace `s.deps = &Deps{}` and the immediate `registerRoutes(mux, s.deps)` with a 2-step init: server-builder first, deps applied, then routes registered. Replace `New()` body:

```go
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

func (s *Server) Serve() error {
	mux := http.NewServeMux()
	registerRoutes(mux, s.deps)
	s.httpSrv = &http.Server{
		Handler:           recover(s.logger, requestLog(s.logger, mux)),
		ReadHeaderTimeout: 5 * time.Second,
	}
	err := s.httpSrv.Serve(s.listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
```

This lets the caller wire deps after `New` and before `Serve`.

Update `server_test.go` health test — no change needed; it still works.

- [ ] **Step 14.5: Add test for create+get+list flow**

Append to `internal/server/server_test.go`:

```go
import (
	"bytes"
	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/clock"
	"github.com/spk/spk-cockpit/internal/eventbus"
	"github.com/spk/spk-cockpit/internal/todo"
	"github.com/spk/spk-cockpit/internal/todo/fakerepo"
	"time"
)

func newTestServer(t *testing.T) (string, func()) {
	sock := filepath.Join(t.TempDir(), "test.sock")
	srv, err := server.New(server.Config{SocketPath: sock})
	require.NoError(t, err)
	tr, gr, er := fakerepo.NewTodo(), fakerepo.NewTag(), fakerepo.NewEvent()
	bus := eventbus.New(8)
	srv.Deps().Todos = todo.NewService(tr, gr, er, clock.NewFake(time.Unix(1700000000, 0)), bus)
	srv.Deps().Tags = gr
	srv.Deps().Bus = bus
	go func() { _ = srv.Serve() }()
	waitForSocket(t, sock)
	return sock, func() { _ = srv.Stop(context.Background()); bus.Close() }
}

func TestServer_CreateAndListTodo(t *testing.T) {
	sock, stop := newTestServer(t)
	defer stop()
	c := udsClient(sock)

	body, _ := json.Marshal(api.CreateTodoRequest{Title: "X", Priority: api.PriorityNormal})
	resp, err := c.Post("http://unix/api/todos", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	require.Equal(t, 201, resp.StatusCode)
	resp.Body.Close()

	resp, err = c.Get("http://unix/api/todos")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode)
	var list []api.Todo
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&list))
	require.Len(t, list, 1)
	require.Equal(t, "X", list[0].Title)
}

func udsClient(sock string) *http.Client {
	return &http.Client{Transport: &http.Transport{
		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", sock)
		},
	}}
}
```

- [ ] **Step 14.6: Run tests**

```bash
go test ./internal/server/... -v
```

Expected: PASS.

- [ ] **Step 14.7: Commit**

```bash
git add .
git commit -m "feat: wire Todo service into UDS server with REST handlers"
```

---

## Task 15: SSE events endpoint

**Files:**
- Create: `internal/server/events_handler.go`
- Modify: `internal/server/routes.go`
- Modify: `internal/server/server_test.go` (add SSE test)

- [ ] **Step 15.1: Write `internal/server/events_handler.go`**

```go
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

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		ch := d.Bus.Subscribe(64)
		defer d.Bus.Unsubscribe(ch)

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
```

- [ ] **Step 15.2: Add SSE route**

Edit `internal/server/routes.go`, add a line before the closing brace of `registerRoutes`:

```go
	mux.HandleFunc("GET /api/events", handleEvents(d))
```

- [ ] **Step 15.3: Add SSE test**

Append to `internal/server/server_test.go`:

```go
func TestServer_SSEReceivesPublishedEvents(t *testing.T) {
	sock, stop := newTestServer(t)
	defer stop()

	c := udsClient(sock)
	req, _ := http.NewRequest("GET", "http://unix/api/events", nil)
	resp, err := c.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode)

	body, _ := json.Marshal(api.CreateTodoRequest{Title: "evt-test", Priority: api.PriorityNormal})
	postResp, err := c.Post("http://unix/api/todos", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	postResp.Body.Close()

	buf := make([]byte, 1024)
	deadline := time.Now().Add(2 * time.Second)
	got := ""
	for time.Now().Before(deadline) && !strings.Contains(got, "todo.created") {
		_ = resp.Body.(interface{ SetReadDeadline(time.Time) error }).SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		n, _ := resp.Body.Read(buf)
		got += string(buf[:n])
	}
	require.Contains(t, got, "todo.created")
}
```

If the type assertion fails (Go's standard `*net/http.Body` does not expose deadline directly), simplify the read loop to a goroutine + channel pattern:

```go
done := make(chan string, 1)
go func() {
	buf := make([]byte, 4096)
	got := ""
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			got += string(buf[:n])
			if strings.Contains(got, "todo.created") {
				done <- got
				return
			}
		}
		if err != nil {
			done <- got
			return
		}
	}
}()
select {
case got := <-done:
	require.Contains(t, got, "todo.created")
case <-time.After(2 * time.Second):
	t.Fatal("no SSE event received")
}
```

Add `"strings"` to the test imports.

- [ ] **Step 15.4: Run server tests**

```bash
go test ./internal/server/... -v
```

Expected: PASS for all three tests.

- [ ] **Step 15.5: Commit**

```bash
git add .
git commit -m "feat: add SSE events endpoint"
```

---

## Task 16: CLI client over UDS

**Files:**
- Create: `internal/cli/client.go`
- Create: `internal/cli/client_test.go`

- [ ] **Step 16.1: Write `internal/cli/client.go`**

```go
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/spk/spk-cockpit/internal/api"
)

type Client struct {
	httpClient *http.Client
}

func NewClient(socketPath string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
					return net.DialTimeout("unix", socketPath, 2*time.Second)
				},
			},
		},
	}
}

func (c *Client) Health(ctx context.Context) error {
	resp, err := c.do(ctx, "GET", "/api/health", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *Client) ListTodos(ctx context.Context, includeDone bool) ([]api.Todo, error) {
	path := "/api/todos"
	if includeDone {
		path += "?includeDone=1"
	}
	var out []api.Todo
	if err := c.getJSON(ctx, path, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) CreateTodo(ctx context.Context, req api.CreateTodoRequest) (api.Todo, error) {
	var out api.Todo
	if err := c.postJSON(ctx, "/api/todos", req, &out); err != nil {
		return api.Todo{}, err
	}
	return out, nil
}

func (c *Client) UpdateTodo(ctx context.Context, id string, req api.UpdateTodoRequest) (api.Todo, error) {
	var out api.Todo
	if err := c.patchJSON(ctx, "/api/todos/"+id, req, &out); err != nil {
		return api.Todo{}, err
	}
	return out, nil
}

func (c *Client) DeleteTodo(ctx context.Context, id string) error {
	resp, err := c.do(ctx, "DELETE", "/api/todos/"+id, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *Client) do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, "http://unix"+path, body)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("daemon connect: %w", err)
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		var er api.ErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&er)
		if er.Error.Message != "" {
			return nil, fmt.Errorf("api: %s (%s)", er.Error.Message, er.Error.Code)
		}
		return nil, fmt.Errorf("api: status %d", resp.StatusCode)
	}
	return resp, nil
}

func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	resp, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) postJSON(ctx context.Context, path string, body, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	resp, err := c.do(ctx, "POST", path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) patchJSON(ctx context.Context, path string, body, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	resp, err := c.do(ctx, "PATCH", path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(out)
}

var ErrDaemonNotRunning = errors.New("daemon not running")
```

- [ ] **Step 16.2: Verify compilation**

```bash
go build ./internal/cli/...
```

- [ ] **Step 16.3: Commit**

```bash
git add .
git commit -m "feat: add CLI HTTP client over UDS"
```

---

## Task 17: CLI `start` command (foreground daemon)

**Files:**
- Create: `internal/cli/start.go`
- Modify: `internal/cli/root.go`

In phase 1 the `start` command runs in the foreground (no fork). It opens the DB, runs migrations, wires services, starts the UDS server, and (in later tasks) opens the Wails window and tray. For task 17, it just runs the server.

- [ ] **Step 17.1: Write `internal/cli/start.go`**

```go
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/spk/spk-cockpit/internal/clock"
	"github.com/spk/spk-cockpit/internal/eventbus"
	"github.com/spk/spk-cockpit/internal/log"
	"github.com/spk/spk-cockpit/internal/paths"
	"github.com/spk/spk-cockpit/internal/server"
	"github.com/spk/spk-cockpit/internal/store"
	"github.com/spk/spk-cockpit/internal/todo"
)

var startFlags struct {
	foreground bool
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the spk-cockpit tray app",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runStart(cmd.Context())
	},
}

func init() {
	startCmd.Flags().BoolVar(&startFlags.foreground, "foreground", false, "Run in foreground (do not fork)")
	rootCmd.AddCommand(startCmd)
}

func runStart(ctx context.Context) error {
	logger := log.New(os.Stderr, log.ParseLevel(os.Getenv("SPK_COCKPIT_LOG_LEVEL")))

	p, err := paths.New()
	if err != nil {
		return fmt.Errorf("paths: %w", err)
	}
	logger.Info("starting spk-cockpit", "data", p.DataDir, "socket", p.SocketFile)

	st, err := store.Open("file:" + p.DBFile)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer st.Close()
	if err := store.Migrate(st.DB); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	bus := eventbus.New(64)
	defer bus.Close()

	todoRepo := store.NewTodoRepo(st.DB)
	tagRepo := store.NewTagRepo(st.DB)
	eventRepo := store.NewEventRepo(st.DB)

	todoSvc := todo.NewService(todoRepo, tagRepo, eventRepo, clock.Real(), bus)

	srv, err := server.New(server.Config{SocketPath: p.SocketFile, Logger: logger})
	if err != nil {
		return fmt.Errorf("server: %w", err)
	}
	srv.Deps().Todos = todoSvc
	srv.Deps().Tags = tagRepo
	srv.Deps().Bus = bus

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go func() {
		<-ctx.Done()
		logger.Info("shutting down")
		_ = srv.Stop(context.Background())
	}()

	logger.Info("server listening", "socket", p.SocketFile)
	if err := srv.Serve(); err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}
```

- [ ] **Step 17.2: Update root command to wire context**

Replace `internal/cli/root.go`:

```go
package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "cockpit",
	Short:         "spk-cockpit — personal productivity tray app",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return rootCmd.ExecuteContext(ctx)
}
```

- [ ] **Step 17.3: Build and smoke-test**

```bash
go build -o /tmp/spk-cockpit ./cmd/cockpit
SPK_COCKPIT_DATA_DIR=/tmp/spk-cockpit-data \
SPK_COCKPIT_STATE_DIR=/tmp/spk-cockpit-state \
SPK_COCKPIT_CONFIG_DIR=/tmp/spk-cockpit-config \
/tmp/spk-cockpit start &
sleep 1
curl --unix-socket /tmp/spk-cockpit-state/cockpit.sock http://unix/api/health
kill %1
```

Expected: `{"status":"ok"}`. Clean up `rm /tmp/spk-cockpit`.

- [ ] **Step 17.4: Commit**

```bash
git add .
git commit -m "feat: implement cockpit start command"
```

---

## Task 18: CLI `stop` command

**Files:**
- Create: `internal/cli/stop.go`

The `stop` command sends a graceful shutdown signal. In phase 1 we don't have a daemon endpoint for shutdown, so we send SIGTERM via PID file (created by `start`). Two changes are needed: (a) `start` writes a pidfile, (b) `stop` reads it.

- [ ] **Step 18.1: Modify `start.go` to write pid file**

In `internal/cli/start.go`, near the top of `runStart` (after `paths.New`), add:

```go
	pidFile := p.StateDir + "/cockpit.pid"
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0o644); err != nil {
		return fmt.Errorf("write pid file: %w", err)
	}
	defer os.Remove(pidFile)
```

- [ ] **Step 18.2: Write `internal/cli/stop.go`**

```go
package cli

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/spk/spk-cockpit/internal/paths"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running spk-cockpit daemon",
	RunE: func(cmd *cobra.Command, _ []string) error {
		p, err := paths.New()
		if err != nil {
			return fmt.Errorf("paths: %w", err)
		}
		pidFile := p.StateDir + "/cockpit.pid"
		raw, err := os.ReadFile(pidFile)
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("daemon is not running")
		}
		if err != nil {
			return fmt.Errorf("read pid file: %w", err)
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
		if err != nil {
			return fmt.Errorf("parse pid: %w", err)
		}
		proc, err := os.FindProcess(pid)
		if err != nil {
			return fmt.Errorf("find process %d: %w", pid, err)
		}
		if err := proc.Signal(syscall.SIGTERM); err != nil {
			return fmt.Errorf("signal: %w", err)
		}
		// wait up to 5s for the process to exit
		for i := 0; i < 50; i++ {
			if proc.Signal(syscall.Signal(0)) != nil {
				fmt.Println("daemon stopped")
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		}
		return errors.New("daemon did not exit after 5s; consider SIGKILL manually")
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
```

- [ ] **Step 18.3: Smoke-test**

```bash
go build -o /tmp/spk-cockpit ./cmd/cockpit
SPK_COCKPIT_DATA_DIR=/tmp/spk-cockpit-data \
SPK_COCKPIT_STATE_DIR=/tmp/spk-cockpit-state \
SPK_COCKPIT_CONFIG_DIR=/tmp/spk-cockpit-config \
/tmp/spk-cockpit start &
sleep 1
SPK_COCKPIT_DATA_DIR=/tmp/spk-cockpit-data \
SPK_COCKPIT_STATE_DIR=/tmp/spk-cockpit-state \
SPK_COCKPIT_CONFIG_DIR=/tmp/spk-cockpit-config \
/tmp/spk-cockpit stop
```

Expected: "daemon stopped". `start &` returns to shell.

- [ ] **Step 18.4: Commit**

```bash
git add .
git commit -m "feat: add cockpit stop command"
```

---

## Task 19: CLI `todo` subcommands

**Files:**
- Create: `internal/cli/todo.go`

- [ ] **Step 19.1: Write `internal/cli/todo.go`**

```go
package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/paths"
)

var todoCmd = &cobra.Command{
	Use:   "todo",
	Short: "Manage todos",
}

var todoAddFlags struct {
	priority string
	tags     []string
	due      string
	notes    string
}

var todoAddCmd = &cobra.Command{
	Use:   "add <title>",
	Short: "Add a new todo",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		title := strings.Join(args, " ")
		req := api.CreateTodoRequest{
			Title:    title,
			Notes:    todoAddFlags.notes,
			Priority: parsePriority(todoAddFlags.priority),
			Tags:     todoAddFlags.tags,
		}
		if todoAddFlags.due != "" {
			ts, err := parseDue(todoAddFlags.due)
			if err != nil {
				return err
			}
			req.DueAt = &ts
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		t, err := c.CreateTodo(context.Background(), req)
		if err != nil {
			return err
		}
		fmt.Printf("created %s: %s\n", t.ID, t.Title)
		return nil
	},
}

var todoListCmd = &cobra.Command{
	Use:   "list",
	Short: "List todos",
	RunE: func(cmd *cobra.Command, _ []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		all, _ := cmd.Flags().GetBool("all")
		todos, err := c.ListTodos(context.Background(), all)
		if err != nil {
			return err
		}
		if len(todos) == 0 {
			fmt.Println("(no todos)")
			return nil
		}
		for _, t := range todos {
			due := ""
			if t.DueAt != nil {
				due = " [due " + time.Unix(*t.DueAt, 0).Format("2006-01-02") + "]"
			}
			tags := ""
			if len(t.Tags) > 0 {
				tags = " #" + strings.Join(t.Tags, " #")
			}
			fmt.Printf("%s [%s] (%s) %s%s%s\n", t.ID[len(t.ID)-6:], priorityStr(t.Priority), t.Status, t.Title, due, tags)
		}
		return nil
	},
}

var todoDoneCmd = &cobra.Command{
	Use:   "done <id-suffix>",
	Short: "Mark a todo as done",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		id, err := resolveID(c, args[0])
		if err != nil {
			return err
		}
		done := api.StatusDone
		_, err = c.UpdateTodo(context.Background(), id, api.UpdateTodoRequest{Status: &done})
		if err != nil {
			return err
		}
		fmt.Println("done")
		return nil
	},
}

var todoRmCmd = &cobra.Command{
	Use:   "rm <id-suffix>",
	Short: "Delete a todo",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		id, err := resolveID(c, args[0])
		if err != nil {
			return err
		}
		if err := c.DeleteTodo(context.Background(), id); err != nil {
			return err
		}
		fmt.Println("deleted")
		return nil
	},
}

func init() {
	todoAddCmd.Flags().StringVarP(&todoAddFlags.priority, "priority", "p", "normal", "low|normal|high|urgent")
	todoAddCmd.Flags().StringSliceVarP(&todoAddFlags.tags, "tag", "t", nil, "tag (repeatable)")
	todoAddCmd.Flags().StringVar(&todoAddFlags.due, "due", "", "YYYY-MM-DD or unix seconds")
	todoAddCmd.Flags().StringVarP(&todoAddFlags.notes, "notes", "n", "", "notes (markdown)")
	todoListCmd.Flags().BoolP("all", "a", false, "include done/cancelled")
	todoCmd.AddCommand(todoAddCmd, todoListCmd, todoDoneCmd, todoRmCmd)
	rootCmd.AddCommand(todoCmd)
}

func newClient() (*Client, error) {
	p, err := paths.New()
	if err != nil {
		return nil, err
	}
	c := NewClient(p.SocketFile)
	if err := c.Health(context.Background()); err != nil {
		return nil, fmt.Errorf("daemon not reachable (run `cockpit start`): %w", err)
	}
	return c, nil
}

func resolveID(c *Client, suffix string) (string, error) {
	todos, err := c.ListTodos(context.Background(), true)
	if err != nil {
		return "", err
	}
	var matches []string
	for _, t := range todos {
		if strings.HasSuffix(t.ID, suffix) {
			matches = append(matches, t.ID)
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no todo matching %q", suffix)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("ambiguous suffix %q matches %d todos", suffix, len(matches))
	}
	return matches[0], nil
}

func parsePriority(s string) api.Priority {
	switch strings.ToLower(s) {
	case "low":
		return api.PriorityLow
	case "high":
		return api.PriorityHigh
	case "urgent":
		return api.PriorityUrgent
	default:
		return api.PriorityNormal
	}
}

func priorityStr(p api.Priority) string {
	switch p {
	case api.PriorityLow:
		return "low"
	case api.PriorityHigh:
		return "high"
	case api.PriorityUrgent:
		return "urgent"
	default:
		return "normal"
	}
}

func parseDue(s string) (int64, error) {
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n, nil
	}
	t, err := time.ParseInLocation("2006-01-02", s, time.Local)
	if err != nil {
		return 0, fmt.Errorf("invalid due value %q (expected YYYY-MM-DD or unix seconds)", s)
	}
	return t.Add(18 * time.Hour).Unix(), nil // default to 18:00 local for date-only
}
```

- [ ] **Step 19.2: Smoke-test**

```bash
go build -o /tmp/spk-cockpit ./cmd/cockpit
export SPK_COCKPIT_DATA_DIR=/tmp/spk-cockpit-data
export SPK_COCKPIT_STATE_DIR=/tmp/spk-cockpit-state
export SPK_COCKPIT_CONFIG_DIR=/tmp/spk-cockpit-config
/tmp/spk-cockpit start &
sleep 1
/tmp/spk-cockpit todo add "First todo" --priority high --tag backend
/tmp/spk-cockpit todo add "Second" --priority normal
/tmp/spk-cockpit todo list
/tmp/spk-cockpit stop
unset SPK_COCKPIT_DATA_DIR SPK_COCKPIT_STATE_DIR SPK_COCKPIT_CONFIG_DIR
```

Expected: `todo add` prints two created IDs, `todo list` shows both with [high] / [normal] priority and `#backend` tag on the first.

- [ ] **Step 19.3: Commit**

```bash
git add .
git commit -m "feat: add cockpit todo add/list/done/rm subcommands"
```

---

## Task 20: React + Vite + Tailwind scaffolding

**Files:** entire `web/` tree

- [ ] **Step 20.1: Initialize web project**

```bash
cd /home/spk/IdeaProjects/spk-task-manager/web
pnpm init
pnpm add -D vite typescript @types/node @vitejs/plugin-react @types/react @types/react-dom \
  tailwindcss postcss autoprefixer eslint @eslint/js typescript-eslint vitest \
  @testing-library/react @testing-library/jest-dom jsdom
pnpm add react react-dom zustand lucide-react
```

- [ ] **Step 20.2: Write `web/package.json` scripts (replace `scripts` block)**

```json
{
  "scripts": {
    "dev": "vite",
    "build": "tsc -b && vite build",
    "preview": "vite preview",
    "test": "vitest",
    "lint": "eslint ."
  }
}
```

- [ ] **Step 20.3: Write `web/tsconfig.json`**

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "lib": ["ES2022", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "moduleResolution": "bundler",
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true,
    "esModuleInterop": true,
    "isolatedModules": true,
    "skipLibCheck": true,
    "resolveJsonModule": true,
    "types": ["vitest/globals", "@testing-library/jest-dom"]
  },
  "include": ["src"]
}
```

- [ ] **Step 20.4: Write `web/vite.config.ts`**

```ts
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      // dev: proxy /api to a running cockpit daemon.
      // For local dev, run `cockpit start` and replace the unix socket with a TCP listener
      // (added in a future phase). For now, treat dev as backend-less and use mocks.
    },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./src/test-setup.ts"],
  },
});
```

- [ ] **Step 20.5: Write `web/postcss.config.js`**

```js
export default {
  plugins: { tailwindcss: {}, autoprefixer: {} },
};
```

- [ ] **Step 20.6: Write `web/tailwind.config.js`**

```js
/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  darkMode: "class",
  theme: {
    extend: {
      colors: {
        bg:        "#1e1e2e",
        bgsub:     "#181825",
        bgmute:    "#11111b",
        fg:        "#cdd6f4",
        fgmute:    "#a6adc8",
        accent:    "#89b4fa",
        urgent:    "#f38ba8",
        high:      "#fab387",
        normal:    "#a6adc8",
        low:       "#6c7086",
        success:   "#a6e3a1",
      },
    },
  },
};
```

- [ ] **Step 20.7: Write `web/index.html`**

```html
<!doctype html>
<html lang="en" class="dark">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>spk-cockpit</title>
  </head>
  <body class="bg-bg text-fg">
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

- [ ] **Step 20.8: Write `web/src/index.css`**

```css
@tailwind base;
@tailwind components;
@tailwind utilities;

html, body, #root { height: 100%; }
body { font-family: system-ui, -apple-system, sans-serif; }
```

- [ ] **Step 20.9: Write `web/src/main.tsx`**

```tsx
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "./index.css";
import { App } from "./App";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
```

- [ ] **Step 20.10: Write `web/src/App.tsx` (placeholder)**

```tsx
import { Todos } from "./pages/Todos";

export function App() {
  return (
    <div className="min-h-screen flex">
      <aside className="w-48 bg-bgsub border-r border-bgmute p-4">
        <h1 className="text-lg font-semibold mb-4">spk-cockpit</h1>
        <nav className="flex flex-col gap-1 text-fgmute">
          <a className="text-fg">Todos</a>
        </nav>
      </aside>
      <main className="flex-1 p-6 overflow-auto">
        <Todos />
      </main>
    </div>
  );
}
```

- [ ] **Step 20.11: Write `web/src/test-setup.ts`**

```ts
import "@testing-library/jest-dom";
```

- [ ] **Step 20.12: Write `web/eslint.config.js` (minimal flat config)**

```js
import js from "@eslint/js";
import tseslint from "typescript-eslint";

export default [
  js.configs.recommended,
  ...tseslint.configs.recommended,
  { ignores: ["dist/", "node_modules/"] },
];
```

- [ ] **Step 20.13: Verify build works (placeholder Todos page)**

Create `web/src/pages/Todos.tsx`:

```tsx
export function Todos() {
  return <div>Todos placeholder</div>;
}
```

Then:

```bash
cd web && pnpm build
```

Expected: `dist/` is created with `index.html` + bundled JS/CSS.

- [ ] **Step 20.14: Commit**

```bash
cd /home/spk/IdeaProjects/spk-task-manager
git add .
git commit -m "feat: scaffold React + Vite + Tailwind web project"
```

---

## Task 21: TypeScript types, API client, SSE client, Zustand store

**Files:**
- Create: `web/src/lib/types.ts`
- Create: `web/src/lib/api.ts`
- Create: `web/src/lib/events.ts`
- Create: `web/src/lib/store.ts`

- [ ] **Step 21.1: Write `web/src/lib/types.ts`** (mirror Go DTOs)

```ts
export type Priority = 0 | 1 | 2 | 3;
export const Priority = { Low: 0, Normal: 1, High: 2, Urgent: 3 } as const;

export type TodoStatus = "open" | "in_progress" | "done" | "cancelled";

export interface Todo {
  id: string;
  title: string;
  notes: string;
  priority: Priority;
  status: TodoStatus;
  dueAt?: number;
  tags: string[];
  createdAt: number;
  updatedAt: number;
  doneAt?: number;
}

export interface Tag {
  name: string;
  color: string;
  createdAt: number;
}

export interface CreateTodoRequest {
  title: string;
  notes?: string;
  priority: Priority;
  dueAt?: number;
  tags?: string[];
}

export interface UpdateTodoRequest {
  title?: string;
  notes?: string;
  priority?: Priority;
  status?: TodoStatus;
  dueAt?: number;
  tags?: string[];
}

export interface ApiEvent<T = unknown> {
  type: string;
  data: T;
}
```

- [ ] **Step 21.2: Write `web/src/lib/api.ts`**

```ts
import type { Todo, Tag, CreateTodoRequest, UpdateTodoRequest } from "./types";

const BASE = ""; // same-origin: served by daemon (or Wails interceptor → UDS)

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const resp = await fetch(BASE + path, {
    headers: { "Content-Type": "application/json" },
    ...init,
  });
  if (!resp.ok) {
    let msg = `HTTP ${resp.status}`;
    try {
      const body = await resp.json();
      if (body?.error?.message) msg = body.error.message;
    } catch {
      // ignore
    }
    throw new Error(msg);
  }
  if (resp.status === 204) return undefined as T;
  return (await resp.json()) as T;
}

export const api = {
  listTodos: (includeDone = false) =>
    request<Todo[]>(`/api/todos${includeDone ? "?includeDone=1" : ""}`),
  createTodo: (req: CreateTodoRequest) =>
    request<Todo>("/api/todos", { method: "POST", body: JSON.stringify(req) }),
  updateTodo: (id: string, req: UpdateTodoRequest) =>
    request<Todo>(`/api/todos/${id}`, { method: "PATCH", body: JSON.stringify(req) }),
  deleteTodo: (id: string) =>
    request<void>(`/api/todos/${id}`, { method: "DELETE" }),
  listTags: () => request<Tag[]>("/api/tags"),
};
```

- [ ] **Step 21.3: Write `web/src/lib/events.ts`**

```ts
import type { ApiEvent } from "./types";

export type EventHandler = (e: ApiEvent) => void;

export class EventStream {
  private es: EventSource | null = null;
  private handlers: EventHandler[] = [];
  private retry = 1000;

  start() {
    this.connect();
  }

  stop() {
    this.es?.close();
    this.es = null;
  }

  on(h: EventHandler) {
    this.handlers.push(h);
    return () => {
      this.handlers = this.handlers.filter((x) => x !== h);
    };
  }

  private connect() {
    this.es = new EventSource("/api/events");
    this.es.onmessage = (ev) => {
      try {
        const data = JSON.parse(ev.data) as ApiEvent;
        for (const h of this.handlers) h(data);
      } catch {
        // ignore malformed
      }
    };
    this.es.onerror = () => {
      this.es?.close();
      setTimeout(() => this.connect(), this.retry);
      this.retry = Math.min(this.retry * 2, 30_000);
    };
    this.es.onopen = () => {
      this.retry = 1000;
    };
  }
}
```

- [ ] **Step 21.4: Write `web/src/lib/store.ts`**

```ts
import { create } from "zustand";
import { api } from "./api";
import type { Todo, ApiEvent } from "./types";

interface TodoState {
  todos: Todo[];
  loading: boolean;
  includeDone: boolean;
  error: string | null;
  load: () => Promise<void>;
  setIncludeDone: (v: boolean) => void;
  applyEvent: (e: ApiEvent) => void;
}

export const useTodoStore = create<TodoState>((set, get) => ({
  todos: [],
  loading: false,
  includeDone: false,
  error: null,
  async load() {
    set({ loading: true, error: null });
    try {
      const todos = await api.listTodos(get().includeDone);
      set({ todos, loading: false });
    } catch (e) {
      set({ error: (e as Error).message, loading: false });
    }
  },
  setIncludeDone(v) {
    set({ includeDone: v });
    void get().load();
  },
  applyEvent(e) {
    if (e.type === "todo.created") {
      const { todo } = e.data as { todo: Todo };
      set({ todos: [todo, ...get().todos] });
    } else if (e.type === "todo.updated") {
      const { todo } = e.data as { todo: Todo };
      set({ todos: get().todos.map((t) => (t.id === todo.id ? todo : t)) });
    } else if (e.type === "todo.deleted") {
      const { todoId } = e.data as { todoId: string };
      set({ todos: get().todos.filter((t) => t.id !== todoId) });
    } else if (e.type === "todo.status_changed") {
      // a full update event will follow; ignore for now
    }
  },
}));
```

- [ ] **Step 21.5: Verify build**

```bash
cd web && pnpm build
```

Expected: success.

- [ ] **Step 21.6: Commit**

```bash
cd /home/spk/IdeaProjects/spk-task-manager
git add .
git commit -m "feat: add web API client, SSE wrapper and Zustand store"
```

---

## Task 22: Todo UI components

**Files:**
- Create: `web/src/components/TodoList.tsx`
- Create: `web/src/components/TodoRow.tsx`
- Create: `web/src/components/AddTodoForm.tsx`
- Create: `web/src/components/TagPill.tsx`
- Modify: `web/src/pages/Todos.tsx`

- [ ] **Step 22.1: Write `web/src/components/TagPill.tsx`**

```tsx
export function TagPill({ name }: { name: string }) {
  return (
    <span className="inline-block px-2 py-0.5 rounded-full bg-bgmute text-fgmute text-xs">
      #{name}
    </span>
  );
}
```

- [ ] **Step 22.2: Write `web/src/components/TodoRow.tsx`**

```tsx
import { Check, Trash2 } from "lucide-react";
import type { Todo } from "../lib/types";
import { Priority } from "../lib/types";
import { TagPill } from "./TagPill";

const priorityClass: Record<number, string> = {
  [Priority.Urgent]: "text-urgent",
  [Priority.High]: "text-high",
  [Priority.Normal]: "text-normal",
  [Priority.Low]: "text-low",
};

const priorityGlyph: Record<number, string> = {
  [Priority.Urgent]: "🔥",
  [Priority.High]: "⚡",
  [Priority.Normal]: "•",
  [Priority.Low]: "▫",
};

export interface TodoRowProps {
  todo: Todo;
  onToggleDone: (todo: Todo) => void;
  onDelete: (todo: Todo) => void;
}

export function TodoRow({ todo, onToggleDone, onDelete }: TodoRowProps) {
  const isDone = todo.status === "done";
  return (
    <div className="flex items-center gap-3 p-3 rounded hover:bg-bgsub group">
      <button
        onClick={() => onToggleDone(todo)}
        className={`w-5 h-5 rounded border ${isDone ? "bg-success border-success" : "border-fgmute"} flex items-center justify-center`}
        aria-label={isDone ? "Mark as open" : "Mark as done"}
      >
        {isDone && <Check size={14} className="text-bg" />}
      </button>
      <span className={priorityClass[todo.priority]}>{priorityGlyph[todo.priority]}</span>
      <span className={`flex-1 ${isDone ? "line-through text-fgmute" : ""}`}>
        {todo.title}
      </span>
      <div className="flex gap-1">
        {todo.tags.map((t) => (
          <TagPill key={t} name={t} />
        ))}
      </div>
      <button
        onClick={() => onDelete(todo)}
        className="opacity-0 group-hover:opacity-100 text-fgmute hover:text-urgent"
        aria-label="Delete"
      >
        <Trash2 size={16} />
      </button>
    </div>
  );
}
```

- [ ] **Step 22.3: Write `web/src/components/AddTodoForm.tsx`**

```tsx
import { useState } from "react";
import { Priority } from "../lib/types";
import { api } from "../lib/api";

export function AddTodoForm() {
  const [title, setTitle] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!title.trim()) return;
    setBusy(true);
    try {
      await api.createTodo({ title: title.trim(), priority: Priority.Normal });
      setTitle("");
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit} className="flex gap-2 items-center">
      <input
        type="text"
        value={title}
        onChange={(e) => setTitle(e.target.value)}
        placeholder="+ Add todo (Enter to submit)"
        className="flex-1 bg-bgsub border border-bgmute rounded px-3 py-2 focus:outline-none focus:border-accent text-fg"
        disabled={busy}
      />
    </form>
  );
}
```

- [ ] **Step 22.4: Write `web/src/components/TodoList.tsx`**

```tsx
import { useEffect } from "react";
import { useTodoStore } from "../lib/store";
import { api } from "../lib/api";
import { TodoRow } from "./TodoRow";
import type { Todo } from "../lib/types";

export function TodoList() {
  const { todos, loading, error, load, includeDone, setIncludeDone } = useTodoStore();

  useEffect(() => {
    void load();
  }, [load]);

  async function toggleDone(t: Todo) {
    const next = t.status === "done" ? "open" : "done";
    await api.updateTodo(t.id, { status: next });
  }

  async function remove(t: Todo) {
    if (!confirm(`Delete "${t.title}"?`)) return;
    await api.deleteTodo(t.id);
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="flex justify-between items-center">
        <h2 className="text-xl font-semibold">Todos</h2>
        <label className="flex items-center gap-2 text-fgmute text-sm">
          <input
            type="checkbox"
            checked={includeDone}
            onChange={(e) => setIncludeDone(e.target.checked)}
          />
          show done
        </label>
      </div>
      {loading && <div className="text-fgmute">loading…</div>}
      {error && <div className="text-urgent">error: {error}</div>}
      <div className="flex flex-col">
        {todos.map((t) => (
          <TodoRow key={t.id} todo={t} onToggleDone={toggleDone} onDelete={remove} />
        ))}
        {!loading && todos.length === 0 && (
          <div className="text-fgmute py-8 text-center">no todos yet</div>
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 22.5: Wire the SSE stream in `Todos.tsx`**

Replace `web/src/pages/Todos.tsx`:

```tsx
import { useEffect } from "react";
import { TodoList } from "../components/TodoList";
import { AddTodoForm } from "../components/AddTodoForm";
import { EventStream } from "../lib/events";
import { useTodoStore } from "../lib/store";

const stream = new EventStream();

export function Todos() {
  const applyEvent = useTodoStore((s) => s.applyEvent);

  useEffect(() => {
    stream.start();
    const off = stream.on(applyEvent);
    return () => {
      off();
      stream.stop();
    };
  }, [applyEvent]);

  return (
    <div className="max-w-2xl flex flex-col gap-6">
      <AddTodoForm />
      <TodoList />
    </div>
  );
}
```

- [ ] **Step 22.6: Verify build**

```bash
cd web && pnpm build
```

Expected: success, `dist/index.html` references hashed JS/CSS bundles.

- [ ] **Step 22.7: Commit**

```bash
cd /home/spk/IdeaProjects/spk-task-manager
git add .
git commit -m "feat: implement Todo list, row, and quick-add components"
```

---

## Task 23: Embed `web/dist` and serve it from the UDS server

**Files:**
- Create: `web/embed.go`
- Modify: `internal/server/routes.go`
- Modify: `internal/server/server.go`

- [ ] **Step 23.1: Write `web/embed.go`**

```go
package web

import "embed"

//go:embed all:dist
var DistFS embed.FS
```

This sits at the project root in `web/embed.go` (note: outside `src/` so Go sees it). Adjust your path: file is `web/embed.go` and the package directive is `package web`. The file does **not** live in `src/`.

- [ ] **Step 23.2: Add static-serving handler**

Append to `internal/server/routes.go`:

```go
import (
	"io/fs"
	"net/http"

	webroot "github.com/spk/spk-cockpit/web"
)

func registerRoutes(mux *http.ServeMux, d *Deps) {
	mux.HandleFunc("GET /api/health", handleHealth)
	mux.HandleFunc("GET /api/todos", handleListTodos(d))
	mux.HandleFunc("POST /api/todos", handleCreateTodo(d))
	mux.HandleFunc("GET /api/todos/{id}", handleGetTodo(d))
	mux.HandleFunc("PATCH /api/todos/{id}", handleUpdateTodo(d))
	mux.HandleFunc("DELETE /api/todos/{id}", handleDeleteTodo(d))
	mux.HandleFunc("GET /api/todos/{id}/history", handleHistoryTodo(d))
	mux.HandleFunc("GET /api/tags", handleListTags(d))
	mux.HandleFunc("GET /api/events", handleEvents(d))

	dist, err := fs.Sub(webroot.DistFS, "dist")
	if err == nil {
		fileServer := http.FileServer(http.FS(dist))
		mux.Handle("/", spaFallback(dist, fileServer))
	}
}

func spaFallback(dist fs.FS, fs http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try the requested file. If 404, fall back to index.html.
		f, err := dist.Open(r.URL.Path[1:])
		if err == nil {
			_ = f.Close()
			fs.ServeHTTP(w, r)
			return
		}
		// serve index.html
		idx, err := dist.Open("index.html")
		if err != nil {
			http.Error(w, "not found", 404)
			return
		}
		defer idx.Close()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeContent(w, r, "index.html", time.Time{}, idx.(io.ReadSeeker))
	})
}
```

Add imports `"io"`, `"time"` if not already present.

- [ ] **Step 23.3: Ensure web build runs before Go build in Makefile**

Already wired in Task 1's Makefile (`build: web-build build-fast`). Verify:

```bash
cd /home/spk/IdeaProjects/spk-task-manager
make build
```

Expected: web builds, then Go builds with embedded assets. Final binary at `build/bin/spk-cockpit`.

- [ ] **Step 23.4: Smoke-test the served UI**

```bash
SPK_COCKPIT_DATA_DIR=/tmp/spk-cockpit-data \
SPK_COCKPIT_STATE_DIR=/tmp/spk-cockpit-state \
SPK_COCKPIT_CONFIG_DIR=/tmp/spk-cockpit-config \
./build/bin/spk-cockpit start &
sleep 1
curl --unix-socket /tmp/spk-cockpit-state/cockpit.sock http://unix/ | head -c 200
./build/bin/spk-cockpit stop
```

Expected: HTML output containing `<div id="root"></div>`.

- [ ] **Step 23.5: Commit**

```bash
git add .
git commit -m "feat: embed web/dist and serve SPA from UDS"
```

---

## Task 24: Wails main window

**Files:**
- Create: `internal/window/window.go`
- Modify: `cmd/cockpit/main.go` (Wails launcher)
- Create: project-root `wails.json`

Wails v2 expects an entry point that calls `wails.Run`. We integrate it as the primary entry point of the CLI's `start` command.

- [ ] **Step 24.1: Add Wails dependencies**

```bash
go get github.com/wailsapp/wails/v2/pkg/runtime
go get github.com/wailsapp/wails/v2
go mod tidy
```

You also need the Wails CLI to scaffold the build pipeline:

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

- [ ] **Step 24.2: Write `wails.json`**

```json
{
  "$schema": "https://wails.io/schemas/config.v2.json",
  "name": "spk-cockpit",
  "outputfilename": "spk-cockpit",
  "frontend:install": "pnpm install --frozen-lockfile",
  "frontend:build": "pnpm build",
  "frontend:dev:watcher": "pnpm dev",
  "frontend:dev:serverUrl": "http://localhost:5173",
  "wailsjsdir": "./web/src",
  "author": { "name": "spk" }
}
```

- [ ] **Step 24.3: Write `internal/window/window.go`**

```go
package window

import (
	"context"
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

type App struct {
	ctx        context.Context
	socketPath string
}

func NewApp(socketPath string) *App {
	return &App{socketPath: socketPath}
}

func (a *App) onStartup(ctx context.Context) {
	a.ctx = ctx
}

func Run(assets embed.FS, socketPath string) error {
	app := NewApp(socketPath)
	return wails.Run(&options.App{
		Title:  "spk-cockpit",
		Width:  1100,
		Height: 720,
		AssetServer: &assetserver.Options{
			Assets: assets,
			Middleware: udsMiddleware(socketPath),
		},
		HideWindowOnClose: true,
		Linux: &linux.Options{
			ProgramName: "spk-cockpit",
		},
		OnStartup: app.onStartup,
	})
}
```

- [ ] **Step 24.4: Write the UDS-passthrough middleware**

Append to `internal/window/window.go`:

```go
import (
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func udsMiddleware(socketPath string) assetserver.Middleware {
	transport := &http.Transport{
		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", socketPath)
		},
	}
	target, _ := url.Parse("http://unix")
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
		},
		Transport: transport,
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(r.URL.Path) >= 5 && r.URL.Path[:5] == "/api/" {
				proxy.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
```

The middleware proxies `/api/*` over the UDS to the in-process Go server, while letting Wails serve the embedded React assets directly. Same pattern as reference-app desktop mode.

- [ ] **Step 24.5: Wire window into `runStart`**

In `internal/cli/start.go`, after `srv.Deps()` is wired and BEFORE `srv.Serve()`, change the `srv.Serve()` block to:

```go
	// Serve in a goroutine; main thread is owned by Wails.
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- srv.Serve()
	}()

	// Run Wails (blocks main thread).
	winErr := window.Run(webroot.DistFS, p.SocketFile)
	logger.Info("window closed", "err", winErr)

	cancel() // request shutdown
	_ = srv.Stop(context.Background())
	if err := <-serveErr; err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	return winErr
```

Add imports:

```go
	"github.com/spk/spk-cockpit/internal/window"
	webroot "github.com/spk/spk-cockpit/web"
```

- [ ] **Step 24.6: Build with Wails CLI and verify the window**

Native Wails dependencies on Linux: `gcc`, `pkg-config`, `libgtk-3-dev`, `libwebkit2gtk-4.0-dev`. Install via your package manager if missing:

```bash
sudo apt install -y gcc pkg-config libgtk-3-dev libwebkit2gtk-4.0-dev
```

Then:

```bash
make web-build
wails build -clean -o spk-cockpit
./build/bin/spk-cockpit start
```

Expected: a window opens displaying the Todos page; you can add a todo, see it appear, mark it done, delete it. Close the window — the daemon shuts down.

If `wails build` doesn't fit the project layout, you can build with vanilla `go build` plus the Wails tags — but Wails docs recommend `wails build` for proper webview2 / GTK linking.

- [ ] **Step 24.7: Commit**

```bash
git add .
git commit -m "feat: add Wails main window with UDS passthrough"
```

---

## Task 25: System tray with basic menu

**Files:**
- Create: `internal/tray/tray.go`
- Create: `internal/tray/tray_linux.go`
- Create: `internal/appfiles/icons.go`
- Create: `icons/tray.png` (22x22)
- Modify: `internal/cli/start.go`

- [ ] **Step 25.1: Provide tray icon**

Place a simple 22×22 PNG at `icons/tray.png`. If you don't have one yet, generate a placeholder:

```bash
cd /home/spk/IdeaProjects/spk-task-manager
mkdir -p icons
# Create a 22x22 transparent-on-blue bullet via ImageMagick (or any image tool):
convert -size 22x22 xc:none -fill "#89b4fa" -draw "circle 11,11 11,3" icons/tray.png
```

If ImageMagick isn't installed: drop in any 22×22 PNG by hand. The contents don't matter functionally for phase 1.

- [ ] **Step 25.2: Write `internal/appfiles/icons.go`**

```go
package appfiles

import _ "embed"

//go:embed ../../icons/tray.png
var TrayIcon []byte
```

- [ ] **Step 25.3: Write `internal/tray/tray.go`**

```go
package tray

type Backend interface {
	Run(onReady func(), onExit func())
	SetTooltip(s string)
	Quit()
}

type MenuItem struct {
	Title   string
	OnClick func()
	Sep     bool
}
```

- [ ] **Step 25.4: Add systray dependency**

```bash
go get fyne.io/systray
go mod tidy
```

- [ ] **Step 25.5: Write `internal/tray/tray_linux.go`**

```go
//go:build linux

package tray

import (
	"fyne.io/systray"

	"github.com/spk/spk-cockpit/internal/appfiles"
)

type linuxTray struct {
	openWindow func()
	quit       func()
}

func New(openWindow, quit func()) Backend {
	return &linuxTray{openWindow: openWindow, quit: quit}
}

func (t *linuxTray) Run(onReady func(), onExit func()) {
	systray.Run(func() {
		systray.SetIcon(appfiles.TrayIcon)
		systray.SetTooltip("spk-cockpit")

		open := systray.AddMenuItem("Open window", "")
		systray.AddSeparator()
		quit := systray.AddMenuItem("Quit", "")

		go func() {
			for {
				select {
				case <-open.ClickedCh:
					if t.openWindow != nil {
						t.openWindow()
					}
				case <-quit.ClickedCh:
					if t.quit != nil {
						t.quit()
					}
					systray.Quit()
					return
				}
			}
		}()

		if onReady != nil {
			onReady()
		}
	}, onExit)
}

func (t *linuxTray) SetTooltip(s string) { systray.SetTooltip(s) }
func (t *linuxTray) Quit()               { systray.Quit() }
```

- [ ] **Step 25.6: Hook tray into `runStart`**

Wails `runtime.WindowShow(ctx)` re-shows the hidden window. Capture the App context and expose a method:

In `internal/window/window.go`, replace `App` and add `Show()`:

```go
import (
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx        context.Context
	socketPath string
}

func NewApp(socketPath string) *App { return &App{socketPath: socketPath} }

func (a *App) onStartup(ctx context.Context) { a.ctx = ctx }

func (a *App) Show() {
	if a.ctx != nil {
		wruntime.WindowShow(a.ctx)
	}
}
```

Update `Run` to return the app pointer so the caller can keep a handle:

```go
func Run(assets embed.FS, socketPath string, ready func(*App)) error {
	app := NewApp(socketPath)
	go func() {
		// Give Wails a moment to call OnStartup
		time.Sleep(200 * time.Millisecond)
		ready(app)
	}()
	return wails.Run(&options.App{
		Title:  "spk-cockpit",
		Width:  1100,
		Height: 720,
		AssetServer: &assetserver.Options{
			Assets:     assets,
			Middleware: udsMiddleware(socketPath),
		},
		HideWindowOnClose: true,
		Linux:             &linux.Options{ProgramName: "spk-cockpit"},
		OnStartup:         app.onStartup,
	})
}
```

Add `"time"` import.

In `internal/cli/start.go`, replace the Wails call site with:

```go
	// Tray runs in its own goroutine; once Wails app is ready, tray clicks call app.Show().
	var winApp *window.App
	winReady := make(chan struct{})

	go func() {
		t := tray.New(
			func() {
				if winApp != nil {
					winApp.Show()
				}
			},
			func() {
				cancel() // request global shutdown
			},
		)
		t.Run(nil, nil)
	}()

	winErr := window.Run(webroot.DistFS, p.SocketFile, func(a *window.App) {
		winApp = a
		close(winReady)
	})
	_ = winReady
	logger.Info("window closed", "err", winErr)
```

Add `"github.com/spk/spk-cockpit/internal/tray"` import.

- [ ] **Step 25.7: Build and smoke-test**

```bash
make build
./build/bin/spk-cockpit start
```

Expected: tray icon appears; right-click shows "Open window" / "Quit"; closing the window keeps the daemon running; "Open window" reopens it; "Quit" exits cleanly.

- [ ] **Step 25.8: Commit**

```bash
git add .
git commit -m "feat: add system tray with open/quit menu"
```

---

## Task 26: Final integration smoke test and README polish

**Files:**
- Modify: `README.md`
- Modify: `Makefile`

- [ ] **Step 26.1: Update `README.md`**

```markdown
# spk-cockpit

Personal productivity tray app — todo list with prioritization, filtering, history, and a single-binary architecture (Go + embedded React UI). Linux first.

## Phase 1 status

- ✅ Todo CRUD (priority, status, due, tags, audit history)
- ✅ Tray icon with menu (Open window / Quit)
- ✅ Wails main window
- ✅ CLI (`cockpit start | stop | todo add/list/done/rm`)
- ✅ SQLite storage with migrations
- ✅ HTTP/UDS server with SSE for realtime UI updates

Phases 2–4 (popover, time-tracking, meetings, CalDAV, standup, secrets, autostart, releases) are planned separately.

## Build

```bash
sudo apt install -y gcc pkg-config libgtk-3-dev libwebkit2gtk-4.0-dev
make build
./build/bin/spk-cockpit start          # opens tray + window
```

## CLI examples

```bash
cockpit todo add "Review MR !1245" -p high -t backend
cockpit todo list
cockpit todo done abc123              # last 6 chars of the ID
cockpit todo rm abc123
cockpit stop
```

## Filesystem

- Database: `~/.local/share/spk-cockpit/cockpit.db`
- Socket: `~/.local/state/spk-cockpit/cockpit.sock`
- Logs: `~/.local/state/spk-cockpit/log/cockpit.log`

Override paths via `SPK_COCKPIT_DATA_DIR`, `SPK_COCKPIT_STATE_DIR`, `SPK_COCKPIT_CONFIG_DIR`.

## Development

```bash
make test        # Go + Vitest
make lint        # golangci-lint + eslint
make fmt
```

In dev mode (web hot-reload), run `wails dev` from the project root.
```

- [ ] **Step 26.2: Verify all tests pass**

```bash
make test
```

Expected: Go tests PASS, Vitest reports either "0 tests" (acceptable for phase 1) or PASS.

- [ ] **Step 26.3: Verify lint passes**

```bash
make tools  # if not yet installed: install golangci-lint v2.x
make lint
```

Fix any warnings (most likely: unused imports from earlier shims like `_unused`, `_ignore`).

- [ ] **Step 26.4: Manual end-to-end check**

```bash
make build
./build/bin/spk-cockpit start
```

In a separate terminal:

```bash
./build/bin/spk-cockpit todo add "Phase 1 done" -p urgent -t milestone
./build/bin/spk-cockpit todo list
```

Verify:
- The new todo appears in the open window **without** any refresh action (SSE working).
- Marking done in the window updates the CLI list output.
- Deleting from the window removes it.
- Closing the window keeps the daemon running (tray icon present).
- "Quit" from the tray menu exits cleanly; `cockpit stop` afterwards reports "daemon is not running".

- [ ] **Step 26.5: Commit final state**

```bash
git add .
git commit -m "docs: update README for phase 1 completion"
```

- [ ] **Step 26.6: Tag the milestone**

```bash
git tag v0.1.0-phase1
```

---

## Phase 1 Done — Definition of Done

- [ ] Single binary at `build/bin/spk-cockpit` runs and shows tray + window.
- [ ] CLI `start` / `stop` / `todo add` / `todo list` / `todo done` / `todo rm` all work.
- [ ] Restarting the daemon preserves todos (SQLite persists between runs).
- [ ] SSE pushes updates from CLI changes into the open window.
- [ ] All tests pass: `make test`.
- [ ] Lint clean: `make lint`.
- [ ] README reflects the actual phase 1 state.
- [ ] git tag `v0.1.0-phase1` created.

When all checkboxes above are ticked, plan **Phase 2: Time-tracking + popover** as the next plan file.
