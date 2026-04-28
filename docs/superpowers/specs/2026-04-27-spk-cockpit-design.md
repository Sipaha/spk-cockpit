# spk-cockpit — Design Spec

**Date:** 2026-04-27
**Status:** Draft, pending user review
**Author:** Brainstorm session

---

## 1. Goal

Personal productivity tray-resident application that helps the user manage their workday from a single always-on UI. Built as a single Go binary with embedded React UI, Linux-first with cross-platform architecture.

### Scope (v1 / MVP)

1. **Todo list** — prioritization, filtering, audit history.
2. **Meeting list** — read-only sync from Yandex Calendar via CalDAV; system notification N minutes before each meeting (default 5, per-meeting override).
3. **Time-tracking on todos** — start/stop timer per todo, daily aggregation.
4. **Meeting notes** — per-meeting markdown notes, persisted with full history.
5. **Daily standup helper** — auto-aggregates "what I did yesterday / today" from closed todos, GitLab commits (read-only), task tracker statuses (read-only). One-click "copy as markdown".

### Out of scope for v1

- Quick Launcher with global hotkey (deferred — architecture leaves room).
- Pomodoro / focus timer.
- End-of-day review, snippets manager.
- Two-way sync with external systems (writes are local-only).
- Habit tracker, mood logging, clipboard history — different product genre.
- Knowledge Base (KB) — planned for next iteration; see §10.

### Non-goals

- Multi-user / shared state. This is strictly a single-user local tool.
- Network-accessible API. Server binds to a Unix Domain Socket only.

---

## 2. High-Level Architecture

Single Go binary `spk-cockpit`. The process owns:

1. **Tray-resident main process** — tray icon, popover window, full window, background workers.
2. **Local HTTP server on UDS** at `~/.local/state/spk-cockpit/cockpit.sock`. Serves REST + SSE for the UI and for the CLI subcommands. No TCP, no auth — UDS provides the security boundary.
3. **Embedded React UI** via `go:embed` of `web/dist`. Two routes/entries: `/popover` (compact) and `/` (full window).
4. **Wails v2 windows** — main full window (hide-on-close) and popover (frameless, positioned at tray icon, hide-on-blur).
5. **System tray** via `fyne.io/systray` (maintained fork of `getlantern/systray`) hidden behind `tray.Backend` interface.
6. **SQLite storage** via `modernc.org/sqlite` (pure Go, no CGO) at `~/.local/share/spk-cockpit/cockpit.db`. Embedded SQL migrations.
7. **Secrets** — AES-256-GCM per-secret encryption; master key from OS keyring via `zalando/go-keyring` (cross-platform: libsecret on Linux, Keychain on macOS, Credential Manager on Windows); fallback to user-provided password prompted on startup and held in process memory only.
8. **Auto-start** — installs systemd-user unit at `~/.config/systemd/user/spk-cockpit.service`. No sudo.
9. **Distribution** — single static binary via `go install` or GitHub Release. No Docker.

**OS-isolation principle:** every OS-specific surface (`tray`, `notify`, `autostart`, future `hotkey`) lives behind a Go interface in `internal/platform/<surface>/`. Currently only Linux implementations exist (DBus/systray/systemd-user). macOS/Windows can be added later by implementing the same interfaces — no rewrite of domain code.

**Transport-agnostic principle:** domain services do not know about HTTP, Wails, or SQLite directly — they accept repository interfaces and a `Clock`. This keeps a future MCP transport (§10) a non-disruptive addition.

---

## 3. Module Layout (`internal/`)

| Package | Purpose |
|---|---|
| `internal/cli/` | Cobra commands: `start` (launch tray), `install --autostart`, `todo add/list/done`, `meeting next`, `standup`, `stop`. All commands except `start` connect to the running daemon over UDS. |
| `internal/server/` | HTTP server on UDS, REST routes, SSE channel, recovery+logging middleware. |
| `internal/api/` | DTOs and event types — Go-side source of truth. TypeScript counterparts maintained by hand in `web/src/lib/types.ts`, mirroring the Go shapes (same approach as reference-app). |
| `internal/store/` | SQLite wrapper, embedded migrations (`embed.FS`), repository interfaces and implementations: `TodoRepo`, `MeetingRepo`, `NoteRepo`, `TimerRepo`, `SecretRepo`, `EventRepo`, `TagRepo`, `KvRepo`, `SyncStateRepo`. |
| `internal/todo/` | Todo domain: create/update/status transitions, priority, filtering, audit-event emission. Pure logic over `TodoRepo` + `EventRepo`. |
| `internal/meeting/` | Meeting domain: model, notification scheduling rules (notify_min override, reset on start_at change), note attachment. |
| `internal/timer/` | Time-tracking: start/pause/stop sessions, single-active-session invariant, daily aggregation reports. |
| `internal/note/` | Notes attached to meetings or todos. Markdown body, version-less in v1. |
| `internal/standup/` | Standup aggregator. Combines closed todos (own data), GitLab commits, the task tracker statuses through source interfaces. |
| `internal/sync/caldav/` | CalDAV client for Yandex (REPORT with `time-range`, parsing via `emersion/go-ical`), periodic syncer, `etag`/`ctag` delta. |
| `internal/sync/gitlab/` | Read-only fetcher of author commits for standup window. |
| `internal/sync/tracker/` | Read-only task tracker fetcher (REST to `/eapps/api/records`). |
| `internal/notify/` | DBus-based system notifications (`godbus/dbus`), throttling, deduplication. Behind interface for future macOS/Windows. |
| `internal/secret/` | AES-256-GCM encrypt/decrypt, master-key acquisition (OS keyring or in-memory password). CRUD over `SecretRepo`. |
| `internal/platform/` | OS-specific implementations: `tray/linux`, `notify/linux`, `autostart/linux` (and reserved `hotkey/linux` for future Quick Launcher). |
| `internal/window/` | Wails wrapper: window creation, popover positioning at tray icon, hide-on-blur, single-instance guard. |
| `internal/config/` | Read/write `~/.config/spk-cockpit/config.yml` (default notify_min, autostart toggle, source enable flags). |
| `internal/log/` | `slog` with human-readable handler (analogous to `CleanLogHandler` in reference-app), rotating file in `~/.local/state/spk-cockpit/log/`. |

**Reserved for future iterations (no v1 code):**

| Package | Purpose (v2+) |
|---|---|
| `internal/kb/` | Knowledge base domain: KB articles, wikilink parsing, graph traversal, markdown analytics. |
| `internal/mcp/` | MCP server transport for agent-driven KB management (and other domain access). |

`web/` directory mirrors reference-app: React 19 + Vite + TypeScript + Tailwind 4 + Zustand + lucide-react. Vite multi-entry: `popover.html`, `index.html`. Built into `web/dist`, embedded into the binary.

---

## 4. Data Model (SQLite)

All timestamps are UTC unix-seconds INTEGER for index-friendly sort/filter. Soft delete via `deleted_at`. Foreign keys enabled with `PRAGMA foreign_keys = ON;` per connection.

```sql
-- todos
CREATE TABLE todos (
  id            TEXT PRIMARY KEY,           -- ulid
  title         TEXT NOT NULL,
  notes         TEXT NOT NULL DEFAULT '',
  priority      INTEGER NOT NULL,           -- 0=low, 1=normal, 2=high, 3=urgent
  status        TEXT NOT NULL,              -- 'open' | 'in_progress' | 'done' | 'cancelled'
  due_at        INTEGER,
  created_at    INTEGER NOT NULL,
  updated_at    INTEGER NOT NULL,
  done_at       INTEGER,
  deleted_at    INTEGER
);
CREATE INDEX idx_todos_status_priority ON todos(status, priority DESC, due_at);
CREATE INDEX idx_todos_done_at         ON todos(done_at) WHERE done_at IS NOT NULL;

-- normalized tags
CREATE TABLE tags (
  name          TEXT PRIMARY KEY,
  color         TEXT NOT NULL DEFAULT '',
  created_at    INTEGER NOT NULL
);

CREATE TABLE todo_tags (
  todo_id       TEXT NOT NULL REFERENCES todos(id) ON DELETE CASCADE,
  tag           TEXT NOT NULL REFERENCES tags(name) ON DELETE CASCADE ON UPDATE CASCADE,
  PRIMARY KEY (todo_id, tag)
);
CREATE INDEX idx_todo_tags_tag ON todo_tags(tag);

-- audit history (todo)
CREATE TABLE todo_events (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  todo_id       TEXT NOT NULL,
  kind          TEXT NOT NULL,              -- 'created' | 'status_changed' | 'priority_changed' | 'edited' | 'deleted' | 'restored'
  from_value    TEXT,
  to_value      TEXT,
  payload       TEXT,                       -- JSON for kind='edited'
  at            INTEGER NOT NULL
);
CREATE INDEX idx_todo_events_todo_at ON todo_events(todo_id, at);

-- meetings
CREATE TABLE meetings (
  id            TEXT PRIMARY KEY,           -- ulid (manual) or sha1(uid+source) (caldav)
  source        TEXT NOT NULL,              -- 'manual' | 'caldav'
  external_uid  TEXT,
  external_etag TEXT,
  title         TEXT NOT NULL,
  description   TEXT NOT NULL DEFAULT '',
  location      TEXT NOT NULL DEFAULT '',
  start_at      INTEGER NOT NULL,
  end_at        INTEGER NOT NULL,
  notify_min    INTEGER,                    -- per-meeting override; NULL → use kv 'meeting.default_notify_min'
  notified_at   INTEGER,                    -- antifire: NULL = not yet notified
  cancelled     INTEGER NOT NULL DEFAULT 0,
  created_at    INTEGER NOT NULL,
  updated_at    INTEGER NOT NULL,
  deleted_at    INTEGER
);
CREATE UNIQUE INDEX uq_meetings_external ON meetings(source, external_uid) WHERE external_uid IS NOT NULL;
CREATE INDEX idx_meetings_start ON meetings(start_at);

-- notes attached to meetings/todos (short-form, NOT KB)
CREATE TABLE notes (
  id            TEXT PRIMARY KEY,           -- ulid
  meeting_id    TEXT,                       -- nullable; concrete FK in v2 if introduced
  todo_id       TEXT,                       -- nullable
  body          TEXT NOT NULL DEFAULT '',   -- markdown
  created_at    INTEGER NOT NULL,
  updated_at    INTEGER NOT NULL,
  deleted_at    INTEGER
);
CREATE INDEX idx_notes_meeting ON notes(meeting_id) WHERE meeting_id IS NOT NULL;
CREATE INDEX idx_notes_todo    ON notes(todo_id)    WHERE todo_id    IS NOT NULL;

-- time-tracking sessions
CREATE TABLE timer_sessions (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  todo_id       TEXT NOT NULL,
  started_at    INTEGER NOT NULL,
  ended_at      INTEGER,                    -- NULL = active
  source        TEXT NOT NULL DEFAULT 'manual'
);
CREATE INDEX idx_timer_todo         ON timer_sessions(todo_id, started_at);
CREATE UNIQUE INDEX uq_timer_active ON timer_sessions(todo_id) WHERE ended_at IS NULL;

-- secrets (AES-256-GCM ciphertext)
CREATE TABLE secrets (
  name          TEXT PRIMARY KEY,           -- 'yandex_caldav', 'gitlab_token', 'tracker_token'
  ciphertext    BLOB NOT NULL,
  nonce         BLOB NOT NULL,
  updated_at    INTEGER NOT NULL
);

-- external sync cursors
CREATE TABLE sync_state (
  source        TEXT PRIMARY KEY,           -- 'caldav' | 'gitlab' | 'tracker'
  cursor        TEXT NOT NULL DEFAULT '',
  last_ok_at    INTEGER,
  last_err      TEXT NOT NULL DEFAULT ''
);

-- generic settings
CREATE TABLE kv (
  k TEXT PRIMARY KEY,
  v TEXT NOT NULL
);
-- e.g. 'meeting.default_notify_min'='5'
```

**Notable decisions:**

1. **Audit history as a separate `todo_events` table** (not a JSON column). Append-only, query-friendly, supports per-event filtering in the History UI.
2. **Idempotent CalDAV import** via `(source, external_uid)` UNIQUE — UPSERT each sync; skip body parsing when `etag` matches.
3. **Notification dedup** via `notified_at`. The scheduler fires only when `notified_at IS NULL AND start_at - notify_min*60 ≤ now`. Restart of the app does not double-fire.
4. **Soft delete** on todos/meetings/notes. A daily GC worker hard-deletes rows older than 30 days.
5. **Single active timer per todo** enforced by partial unique index. Application-level enforcement adds: globally only one active timer at a time (`timer.Start` stops the current one first).
6. **No FTS5 in v1.** Search uses `LIKE`. FTS5 will be introduced when KB lands (§10).

**Future tables (not in v1):** `kb_articles`, `relations(src_kind, src_id, dst_kind, dst_id, type)` for the agent-managed graph (§10).

---

## 5. Data Flow & Events

Pattern: single writer per resource + in-process event bus.

```
UI (popover/full)        CLI (spk-cockpit todo add ...)
       │                              │
       └──────► HTTP/UDS server ◄─────┘
                       │
                       ▼
              Domain services
       (todo, meeting, timer, note, standup)
                       │
        ┌──────────────┼──────────────┐
        ▼              ▼              ▼
    SQLite store    EventBus       Notifier
                       │
                       ▼
                 SSE channel ──► all subscribed UIs
```

**Mutation pipeline.** All writes go through a domain service. Direct UI → SQLite is forbidden. Each mutating service method does, in one transaction: (a) update state, (b) append audit event where applicable, (c) publish a domain event to the bus.

**EventBus** — in-memory `chan Event` + subscriber list. Subscribers in v1:

- **SSE broadcaster** — pushes JSON events to `/api/events`; UIs render reactively.
- **NotificationScheduler** — on `MeetingUpserted` recalculates notification timing; on `MeetingDeleted` cancels.
- **TrayBadge** — updates tray icon and tooltip on changes to overdue todos / next meeting.
- **Logger** — slog debug line per event.

**Event schema** (single source of truth in `internal/api/events.go`):

```
TodoCreated { todo }
TodoUpdated { todo, changedFields[] }
TodoStatusChanged { todoId, from, to }
TodoDeleted { todoId }
MeetingUpserted { meeting }
MeetingDeleted { meetingId }
MeetingNotificationFired { meetingId }
TimerStarted { todoId, sessionId, startedAt }
TimerStopped { todoId, sessionId, endedAt, durationSec }
NoteUpserted { noteId, meetingId?, todoId? }
SyncStateChanged { source, status, lastErr? }
```

**Background workers** (each goroutine, stop on `ctx.Done()`):

| Worker | Schedule | Behavior |
|---|---|---|
| `caldav.Syncer` | every 5 min + manual refresh | Pulls events from Yandex CalDAV (range −7d → +30d), UPSERTs into `meetings`, publishes events. Uses `ctag` for delta. |
| `notify.MeetingScheduler` | tick every 30s | Scans `meetings WHERE start_at - notify_min*60 ≤ now AND notified_at IS NULL AND NOT cancelled`, fires DBus notification, sets `notified_at`. |
| `gitlab.Fetcher` | on demand (standup open) | Caches author commits in last 24h in `kv` under `cache.gitlab.commits.<date>`, TTL 10 min. |
| `tracker.Fetcher` | on demand | Same pattern, PT statuses for the day. |
| `db.GcWorker` | daily at 03:00 local | Hard-deletes soft-deleted rows older than 30 days. |

**Important rules:**

1. **CalDAV is source of truth for synced events.** Manual deletion in Yandex marks the row `cancelled=1`, not hard-delete (preserves orphaned notes). Manual meetings (`source='manual'`) are untouched by syncer.
2. **NotificationScheduler does not back-fire.** If a meeting was moved earlier such that its notification time is now in the past, no notification is sent. If moved later, `notified_at` is reset to NULL via `MeetingService.ResetNotificationIfStartChanged` and the notification fires anew at the right time.
3. **SSE is single-stream** at `/api/events`. UI subscribes on mount, exponential backoff reconnect. On reconnect, UI does a full REST reload — no event replay buffer.
4. **CLI commands hit the running daemon** through the same UDS server, so they use the same domain services and produce the same events. No second SQLite path.
5. **Single-instance guard.** On startup, attempt connect to the UDS and `GET /api/health`. If alive, exit with "already running". If stale (file exists, no responder), unlink and start.

---

## 6. UI

Single React codebase under `web/`. Two routes/entries served from the same bundle.

### 6.1 Popover (`/popover`) — ~360×500 px

Triggered by tray icon click; positioned next to icon; hide-on-blur and Esc.

Sections (top to bottom):
- **Active timer banner** (visible only when a timer runs): elapsed time + todo title + stop button.
- **Next meeting card**: title, "in N min", actions [Open notes] [Snooze].
- **Today todos**: counts (`open / done`), 3–5 visible rows with priority icon, title, start/stop timer button.
- **Quick add todo**: inline input with parsable syntax — `Fix login bug !urgent #backend due:tomorrow` (priority/tag/due embedded).
- **Footer**: link to "Open full window".

Live-updated via SSE.

### 6.2 Full window (`/`) — sidebar + main + right panel

Routes:

| Route | Content |
|---|---|
| `/todos` | List with filters (status / priority / tag / due / fulltext), sortable, drag-drop priority reordering within group. Selection opens detail in right panel: notes, history, timer log. |
| `/calendar` | Day / week view of meetings. Click → right panel: per-meeting note (markdown), notify_min override, "Open in Yandex Calendar" deep-link. |
| `/notes` | All notes with source filter (meeting / todo / standalone). `LIKE`-based fulltext for v1. |
| `/standup` | Three columns: Yesterday / Today / Blockers. Auto-aggregated from closed todos + GitLab + PT. "Copy as markdown" button. |
| `/history` | Audit-event feed (todos), grouped by day; filter by event kind and todo. |
| `/settings` | CalDAV credentials, GitLab token, PT token, default notify_min, autostart toggle, DB import/export. |

### 6.3 Tech choices

- **State:** Zustand stores split by domain: `todos`, `meetings`, `notes`, `timer`, `events`.
- **Routing:** `react-router` for full window; popover is a separate Vite entry, no router needed.
- **Styles:** Tailwind 4, dark theme (Lens/Darcula palette).
- **Icons:** `lucide-react`.
- **Markdown:** `react-markdown` + `remark-gfm`.
- **DnD:** `@dnd-kit/core`.

---

## 7. Error Handling & Resilience

**Principle:** any network/OS/external dependency can fail. Failure is isolated to the affected feature, surfaced to the user, and retried with backoff.

### External integrations

- Each sync worker catches errors, writes to `sync_state.last_err`, publishes `SyncStateChanged`, never panics. Settings UI shows "Last sync: failed N min ago — <reason>" with click-to-detail.
- **Backoff schedule:** 30s → 1m → 5m → 15m, reset on success.
- **Timeouts:** 30s for CalDAV REPORT, 15s for GitLab/PT GET, all via `context.WithTimeout`.
- **Auth (401/403) vs network/5xx:** auth failures stop retry, mark `last_err='auth'`, prompt "Re-enter password" in UI; network/5xx retry with backoff.

### Notifications

- DBus delivery failure (no session, service unavailable) falls back to OS log + tray red dot with reason. Never blocks the scheduler.
- `notified_at` deduplicates across restarts.

### SQLite

- All mutations transactional. Failed transaction → no state change, no event, no UI update.
- `PRAGMA journal_mode=WAL`, `synchronous=NORMAL` per connection.
- `PRAGMA foreign_keys=ON` per connection (mandatory in SQLite).
- Sequential migrations `0001_init.sql`, `0002_*.sql`, applied under `BEGIN EXCLUSIVE`. Migration failure → app refuses to start, error in stderr+log.
- Pre-migration backup: `cp cockpit.db cockpit.db.bak.<version>`.

### Windows (Wails)

- Single-instance guard via UDS health check.
- `ErrorBoundary` at React root with "Reload window" button.
- Workers run in separate goroutines with `defer recover()`; main goroutine stays UI-only.

### Self-error reporting

On critical failures (DB unreachable, migration broken, decrypt failure) — DBus notification "spk-cockpit: storage error, see logs" + tray icon switches to error state. User notices even if no UI is open.

### Time zones

- Storage in UTC unix-seconds. UI renders in OS local TZ.
- CalDAV events with floating time interpreted as local TZ at import; concrete TZ events normalized via `time.LoadLocation`.
- OS TZ change is automatically reflected in UI; notification timing is computed against UTC `start_at` — TZ-independent.

### Secrets

- Master key from OS keyring (`org.freedesktop.secrets`).
- Keyring unavailable → prompt user for password at startup, hold key in process memory only.
- Decrypt failure for one secret → that sync fails, others (todo/timer/notes) continue.

---

## 8. Testing

Domain logic tested without DB or network; integrations tested with real dependencies where possible; UI tested with Vitest + Playwright.

| Layer | Tools | Coverage |
|---|---|---|
| Domain (`todo`, `meeting`, `timer`, `note`, `standup`) | Go + `testify`, fake in-memory repos | Status transitions, priority, timer math, standup aggregation, notification antifire. |
| Store (SQLite) | Go + `testify`, `t.TempDir()` | Migrations, FK cascades, partial unique indexes (active timer), UPSERT idempotency. |
| Sync (CalDAV/GitLab/Tracker) | Go + `httptest.Server` | Response parsing, ctag/etag delta, 401/5xx handling, exponential backoff (via injected clock). |
| Notify scheduler | Go + fake DBus + fake clock | Fires at `start_at - notify_min*60`; `notified_at` blocks repeat; forward-shift resets. |
| HTTP/UDS server | Go + `httptest` over UDS | Routes, DTO shape, SSE broadcaster connect/disconnect/reconnect. |
| Web UI components | Vitest + Testing Library | Components, Zustand stores, quick-add inline parser. |
| E2E | Playwright | Critical flows: create todo → start timer → close → see in history; meeting +6 min → notification; CalDAV 401 → settings shows re-auth prompt. |

**Key techniques:**

1. **Clock injection.** Every domain service and worker takes a `Clock` interface. Tests use a `fakeClock` with manual `Advance(d)`.
2. **Fake repos for domain.** In-memory map repos in `internal/<domain>/internal/fakerepo`. Conformance tests run on both fake and SQLite repos to keep them in sync.
3. **CalDAV fixtures.** Real (obfuscated) Yandex `.ics` payloads in `testdata/caldav/`. Catches Yandex-specific quirks (TZ, recurrences, deletions) that synthetic fixtures miss.
4. **DBus fake in CI** (no graphical session). Real DBus delivery checked locally via `make test-notify-real`.
5. **Race detector in CI** — `go test -race ./...`.
6. **Coverage target:** 70% on domain packages; the rest opportunistic.

**Not unit-tested:** Wails windows, tray icon, real DBus. Smoke-checked on app start, verified manually during development.

### CI

GitHub Actions:
- `go vet`, `golangci-lint` (pinned version, as in reference-app), `go test -race`, `pnpm vitest run`, build.
- Playwright job on `ubuntu-latest` with Xvfb, runs against `vite preview`-served `web/dist` (Wails not needed for web-side E2E).
- Release workflow on `v*.*.*` tag builds `linux/amd64` and `linux/arm64`.

---

## 9. Configuration & Operations

### Config file

`~/.config/spk-cockpit/config.yml` — minimal in v1; most settings live in `kv` table for runtime changes.

```yaml
ui:
  theme: dark
sync:
  caldav:
    enabled: true
    url: https://caldav.yandex.ru/
    username: ""        # set via UI
    # password is in secrets store, not here
    interval_minutes: 5
  gitlab:
    enabled: false
    base_url: ""
    author_username: ""
  tracker:
    enabled: false
    base_url: ""
log:
  level: info           # debug | info | warn | error
```

### KV-stored settings

- `meeting.default_notify_min` = `5`
- `ui.popover.position` = `tray-anchor`
- `gc.last_run_at`

### Filesystem layout

```
~/.config/spk-cockpit/config.yml                      (config)
~/.local/share/spk-cockpit/cockpit.db                 (SQLite)
~/.local/share/spk-cockpit/cockpit.db.bak.<ver>       (pre-migration backups)
~/.local/state/spk-cockpit/cockpit.sock               (UDS)
~/.local/state/spk-cockpit/log/cockpit.log            (slog output)
~/.config/systemd/user/spk-cockpit.service            (autostart unit, optional)
```

### Distribution

- One static binary, `linux/amd64` + `linux/arm64` released via GitHub Releases.
- `go install github.com/spk/spk-cockpit/cmd/cockpit@latest` for source-driven install.
- `spk-cockpit install --autostart` writes the systemd-user unit and enables it.

---

## 10. Future iterations: agent-managed Knowledge Base

A future iteration adds an Obsidian-like Knowledge Base, **managed entirely by agents** (LLMs read/write/link), with a graph of typed relations between articles.

**Not implemented in v1.** This section documents what the v1 architecture must keep open.

### Conceptual scope

- Distinct from the existing `notes` table. `notes` remain short attached memos for meetings/todos.
- KB articles are long-form markdown documents with title, slug, frontmatter, body.
- Articles are linked by typed edges (`references`, `derives_from`, `contradicts`, `implements`, ...). Edges form a navigable graph: backlinks, neighbors, BFS for "related".
- Agents create and curate the graph; the user browses it.

### Architectural slots reserved in v1

1. **Module paths** `internal/kb/` and `internal/mcp/` are reserved (no v1 code).
2. **Domain layer is transport-agnostic** — services know nothing about HTTP, Wails, or storage backend specifics. An MCP server (second transport) can be added later without touching domain code.
3. **Schema room** — `notes` is intentionally not overloaded with KB semantics. The KB iteration will add separate tables: `kb_articles(id, slug, title, frontmatter, body, …)` and `relations(src_kind, src_id, dst_kind, dst_id, type, created_at)`.
4. **HTTP API rules** anticipating agent use:
   - Mutating endpoints idempotent where natural (PUT/PATCH same body → same result), so agents can retry safely.
   - Errors are structured: `{ "error": { "code": "todo.not_found", "message": "..." } }`. Parse-friendly for agents.
   - Bulk endpoints introduced only when needed (YAGNI), but the REST style (`POST /todos/batch`) is kept consistent so future bulk endpoints fit cleanly.
5. **FTS5 is deferred but planned.** When KB lands, `LIKE`-based search will not scale; FTS5 shadow tables for `kb_articles.body` and existing `notes.body` will be added in a migration.

### What is explicitly not done in v1

- No generic `relations` table.
- No generic audit-log over `todo_events`.
- No MCP server.
- No FTS5.
- No `/kb` UI route or graph visualization.
- No agent integration of any kind.

All of the above belong to the KB iteration.

---

## 11. Open questions

None blocking the implementation plan. Items to revisit if user pushes back:

1. CalDAV sync window of −7d → +30d — adequate for "near-term work", not for archival. KB iteration may need a wider read.
2. Quick-add inline syntax (`!urgent #backend due:tomorrow`) chosen over GUI pickers for input speed; revisit if the parser proves brittle.
3. Pre-migration full-DB copy — simple and safe; revisit if DB grows large enough that copy time matters.

---

## 12. Glossary

- **UDS** — Unix Domain Socket. Filesystem-permissioned IPC, no TCP exposure.
- **CalDAV** — RFC 4791 calendar protocol. Yandex Calendar exposes it at `caldav.yandex.ru`.
- **MCP** — Model Context Protocol. Used here as a future agent-facing transport into spk-cockpit.
- **PT** — task tracker. The user's task tracker, queried read-only for standup aggregation.
- **ctag/etag** — CalDAV change tokens at collection / resource level, used for delta sync.
