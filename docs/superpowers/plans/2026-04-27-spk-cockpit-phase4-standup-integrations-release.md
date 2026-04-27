# spk-cockpit Phase 4: Standup helper + GitLab/Tracker integrations + Autostart + Release

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Daily-standup aggregator that compiles "Yesterday / Today / Blockers" from closed todos + GitLab commits + Citeck Project Tracker statuses; web `/standup` route with "Copy as markdown"; CLI `cockpit standup` printing the same markdown to stdout; `cockpit install --autostart` installing a systemd-user unit; and a GitHub Actions release workflow that builds `linux/amd64`+`linux/arm64` and publishes a Release on `v*.*.*` tags.

**Architecture:** New domain `internal/standup/` aggregates from three sources via interfaces. **GitLab** and **Tracker** sources live under `internal/sync/{gitlab,tracker}/` as `Source` interfaces with HTTP impls (`net/http` + bearer token from `secret.Service`) and in-memory fakes. Standup is **on-demand** (no worker): handler `GET /api/standup?date=YYYY-MM-DD` calls `standup.Service.Generate(ctx, day)`, which fans out to the three sources in parallel and assembles `api.StandupReport`. The same path serves CLI `cockpit standup` (prints `MarkdownReport(report)`). Autostart is a Linux-only platform package writing `~/.config/systemd/user/spk-cockpit.service` and running `systemctl --user enable --now`. Release workflow uses GoReleaser-style steps (matrix build, upload to release).

**Tech Stack:** Go 1.26 (existing), `net/http` for GitLab/Tracker (no new deps), React 19 + Vite 8 + Tailwind 4 (existing). No new runtime deps; only new dev infra is GitHub Actions.

**Reference plans:** Phase 1–3 plans in `docs/superpowers/plans/`. Phase 4 builds directly on Phase 3 (commit `b813857`, tag `v0.3.0-phase3`).

**Phase 1–3 conventions still apply:**

- Build tags `webkit2_41 production` for Wails build (`make build`).
- Doc comments on every exported entity (revive). `//nolint:revive` for `pkg.PkgType` stutter is acceptable with explanation.
- No `Co-Authored-By` in commit messages. **Always create new commits, never amend.**
- Don't create binaries in project root (root `.gitignore` ignores `/cockpit` and `/spk-cockpit`).
- All mutations go through a domain service that emits an audit event (where applicable) AND publishes a domain event to the bus. (Standup is read-only — no mutations, but it still emits no events.)
- All repos have a SQLite implementation AND an in-memory fake; conformance tests run identical assertions against both. (Phase 4 adds no new repos — sync_state is reused.)
- IDs — ULID via `github.com/oklog/ulid/v2`.

---

## Out of Scope for Phase 4

- Two-way write to GitLab or Tracker. Phase 4 sources are **read-only**.
- Tracker comment/status mutation from cockpit. Standup only **reads** issue activity.
- macOS/Windows autostart. Linux-only via systemd-user. (Interface `autostart.Backend` is reserved for future OSes.)
- Standup persistence / history. Each `Generate` call recomputes from sources; nothing is stored.
- Webhooks / push from GitLab/Tracker. Pull-only on demand.
- Settings UI for GitLab/Tracker config. Phase 4 wires CLI `cockpit secret set gitlab_token=...` + KV (`gitlab.url`, `gitlab.author_username`, `tracker.url`, `tracker.username`); Settings UI extension is YAGNI for v1.
- Tray "next standup" indicator. Tray stays as-is.
- `cockpit install --uninstall` (removal). Documented manual `systemctl --user disable spk-cockpit` is enough for v1.
- Auto-update / self-update. The release workflow publishes binaries; users download manually.

---

## File Structure (new + modified)

```
spk-task-manager/
├── .github/
│   └── workflows/
│       └── release.yml                                # NEW
├── internal/
│   ├── api/
│   │   ├── dto.go                                     # MODIFIED (StandupReport, StandupItem, GitLabCommit, TrackerItem DTOs)
│   │   └── events.go                                  # (no change in Phase 4)
│   ├── standup/                                       # NEW
│   │   ├── service.go
│   │   ├── markdown.go
│   │   ├── service_test.go
│   │   └── markdown_test.go
│   ├── sync/
│   │   ├── gitlab/                                    # NEW
│   │   │   ├── source.go                              # Source interface + Commit type
│   │   │   ├── http.go                                # HTTP impl
│   │   │   ├── http_test.go                           # uses httptest.Server
│   │   │   └── fake.go                                # in-memory fake
│   │   └── tracker/                                   # NEW
│   │       ├── source.go                              # Source interface + TrackerItem type
│   │       ├── http.go                                # HTTP impl (Citeck PT records query)
│   │       ├── http_test.go                           # uses httptest.Server
│   │       └── fake.go                                # in-memory fake
│   ├── platform/
│   │   └── autostart/                                 # NEW
│   │       ├── autostart.go                           # Backend interface
│   │       ├── linux.go                               # systemd-user impl (build-tagged linux)
│   │       ├── linux_test.go
│   │       └── noop.go                                # other-OS fallback (build-tagged !linux)
│   ├── server/
│   │   ├── server.go                                  # MODIFIED (Deps gains Standup *standup.Service)
│   │   ├── routes.go                                  # MODIFIED (1 new route)
│   │   ├── standup_handler.go                         # NEW
│   │   └── standup_handler_test.go                    # NEW
│   └── cli/
│       ├── client.go                                  # MODIFIED (Standup method)
│       ├── start.go                                   # MODIFIED (wire gitlab/tracker sources + standup service)
│       ├── standup.go                                 # NEW (cobra subcommand)
│       └── install.go                                 # NEW (cobra subcommand: install --autostart)
└── web/
    └── src/
        ├── App.tsx                                    # MODIFIED (add /standup route + nav link)
        ├── lib/
        │   ├── types.ts                               # MODIFIED (StandupReport, StandupItem types)
        │   └── api.ts                                 # MODIFIED (fetchStandup method)
        └── pages/
            └── Standup.tsx                            # NEW (3-column layout + Copy button)
```

After Phase 4: `make build` still produces a single Linux binary `build/bin/spk-cockpit` with `webkit2_41 production` tags. New optional runtime config (in KV / secrets): `gitlab.url`, `gitlab.author_username`, `gitlab_token`, `tracker.url`, `tracker.username`, `tracker_token`. All optional — missing config disables that source silently.

---

## Task 1: API DTOs for standup

**Files:**
- Modify: `internal/api/dto.go`

- [ ] **Step 1.1: Add Standup DTOs at end of `internal/api/dto.go`**

```go
// StandupSection categorizes a standup item.
type StandupSection string

// Standup section labels.
const (
	StandupSectionYesterday StandupSection = "yesterday"
	StandupSectionToday     StandupSection = "today"
	StandupSectionBlockers  StandupSection = "blockers"
)

// StandupItemSource identifies where an item originated.
type StandupItemSource string

// Standup item sources.
const (
	StandupSourceTodo    StandupItemSource = "todo"
	StandupSourceGitLab  StandupItemSource = "gitlab"
	StandupSourceTracker StandupItemSource = "tracker"
)

// StandupItem is one row in a standup report (todo, commit, or tracker activity).
type StandupItem struct {
	Source  StandupItemSource `json:"source"`
	Title   string            `json:"title"`
	Detail  string            `json:"detail,omitempty"`
	URL     string            `json:"url,omitempty"`
	RefID   string            `json:"refId,omitempty"`
	At      int64             `json:"at"`
}

// StandupReport is the full standup payload returned by GET /api/standup.
type StandupReport struct {
	Day       string        `json:"day"`       // YYYY-MM-DD (local TZ of generation)
	Yesterday []StandupItem `json:"yesterday"`
	Today     []StandupItem `json:"today"`
	Blockers  []StandupItem `json:"blockers"`
	Errors    []string      `json:"errors,omitempty"` // per-source warnings, e.g. "gitlab: 401"
}
```

- [ ] **Step 1.2: Verify**

```bash
go build ./...
```

Expected: clean build.

- [ ] **Step 1.3: Commit**

```bash
git add internal/api/dto.go
git commit -m "feat: add standup DTOs (report, item, sources)"
```

---

## Task 2: GitLab Source — interface, fake, HTTP impl

**Files:**
- Create: `internal/sync/gitlab/source.go`
- Create: `internal/sync/gitlab/fake.go`
- Create: `internal/sync/gitlab/http.go`
- Create: `internal/sync/gitlab/http_test.go`

- [ ] **Step 2.1: Create `internal/sync/gitlab/source.go`**

```go
// Package gitlab is a read-only client for fetching the current user's recent
// commits, used by the standup aggregator. No write APIs are exposed.
package gitlab

import (
	"context"
	"errors"
	"time"
)

// ErrNotConfigured is returned when GitLab is not configured (missing URL/token/author).
var ErrNotConfigured = errors.New("gitlab: not configured")

// Commit is a minimal commit record for standup display.
type Commit struct {
	SHA     string    // 40-char hash
	Title   string    // commit message subject (first line)
	URL     string    // https://gitlab.example.com/group/proj/-/commit/<sha>
	Project string    // "group/project"
	At      time.Time // commit timestamp (UTC)
}

// Source fetches commits authored by a configured user in a time window.
type Source interface {
	// CommitsBy returns commits authored by `author` between `since` (inclusive) and
	// `until` (exclusive). Returned commits are sorted DESC by At.
	CommitsBy(ctx context.Context, author string, since, until time.Time) ([]Commit, error)
}
```

- [ ] **Step 2.2: Create `internal/sync/gitlab/fake.go`**

```go
package gitlab

import (
	"context"
	"sort"
	"sync"
	"time"
)

// Fake is an in-memory Source for tests. Safe for concurrent use.
type Fake struct {
	mu      sync.Mutex
	commits []Commit
	err     error
}

// NewFake returns an empty Fake.
func NewFake() *Fake { return &Fake{} }

// SetCommits replaces the canned commit list.
func (f *Fake) SetCommits(cs []Commit) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commits = append([]Commit(nil), cs...)
}

// SetError sets a sticky error returned by every CommitsBy call.
func (f *Fake) SetError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

// CommitsBy filters the canned list by author and range.
func (f *Fake) CommitsBy(_ context.Context, author string, since, until time.Time) ([]Commit, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	out := make([]Commit, 0, len(f.commits))
	_ = author // fake ignores author filter; tests pre-populate matching commits
	for _, c := range f.commits {
		if (c.At.Equal(since) || c.At.After(since)) && c.At.Before(until) {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At.After(out[j].At) })
	return out, nil
}
```

- [ ] **Step 2.3: Create `internal/sync/gitlab/http.go`**

```go
package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Config configures a HTTPSource.
type Config struct {
	BaseURL string // e.g. https://gitlab.example.com
	Token   string // personal access token with read_api
	Timeout time.Duration
}

// HTTPSource calls the GitLab REST v4 API to find the user's commits.
type HTTPSource struct {
	cfg    Config
	client *http.Client
}

// NewHTTPSource constructs a HTTPSource. Returns ErrNotConfigured if BaseURL or Token is empty.
func NewHTTPSource(cfg Config) (*HTTPSource, error) {
	if strings.TrimSpace(cfg.BaseURL) == "" || strings.TrimSpace(cfg.Token) == "" {
		return nil, ErrNotConfigured
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 15 * time.Second
	}
	return &HTTPSource{cfg: cfg, client: &http.Client{Timeout: cfg.Timeout}}, nil
}

// CommitsBy resolves the author username to a user ID and fetches their recent push events
// in [since, until). Each push event yields one Commit per pushed commit.
func (h *HTTPSource) CommitsBy(ctx context.Context, author string, since, until time.Time) ([]Commit, error) {
	uid, err := h.resolveUserID(ctx, author)
	if err != nil {
		return nil, fmt.Errorf("resolve user: %w", err)
	}
	events, err := h.fetchEvents(ctx, uid, since, until)
	if err != nil {
		return nil, fmt.Errorf("fetch events: %w", err)
	}
	out := make([]Commit, 0, len(events))
	for _, e := range events {
		if e.PushData == nil || e.PushData.CommitTitle == "" {
			continue
		}
		out = append(out, Commit{
			SHA:     e.PushData.CommitTo,
			Title:   e.PushData.CommitTitle,
			Project: e.ProjectFullPath,
			URL:     fmt.Sprintf("%s/%s/-/commit/%s", strings.TrimRight(h.cfg.BaseURL, "/"), e.ProjectFullPath, e.PushData.CommitTo),
			At:      e.CreatedAt,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At.After(out[j].At) })
	return out, nil
}

func (h *HTTPSource) resolveUserID(ctx context.Context, username string) (int, error) {
	u, err := url.Parse(strings.TrimRight(h.cfg.BaseURL, "/") + "/api/v4/users")
	if err != nil {
		return 0, err
	}
	q := u.Query()
	q.Set("username", username)
	u.RawQuery = q.Encode()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	req.Header.Set("PRIVATE-TOKEN", h.cfg.Token)
	resp, err := h.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("status %d", resp.StatusCode)
	}
	var users []struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return 0, err
	}
	if len(users) == 0 {
		return 0, errors.New("user not found")
	}
	return users[0].ID, nil
}

type gitlabEvent struct {
	CreatedAt       time.Time `json:"created_at"`
	ActionName      string    `json:"action_name"`
	ProjectFullPath string    `json:"-"`
	ProjectID       int       `json:"project_id"`
	PushData        *struct {
		CommitTo    string `json:"commit_to"`
		CommitTitle string `json:"commit_title"`
		Ref         string `json:"ref"`
	} `json:"push_data,omitempty"`
}

func (h *HTTPSource) fetchEvents(ctx context.Context, userID int, since, until time.Time) ([]gitlabEvent, error) {
	u, _ := url.Parse(fmt.Sprintf("%s/api/v4/users/%d/events", strings.TrimRight(h.cfg.BaseURL, "/"), userID))
	q := u.Query()
	q.Set("action", "pushed")
	q.Set("after", since.UTC().Format("2006-01-02"))
	q.Set("before", until.UTC().Format("2006-01-02"))
	q.Set("per_page", "100")
	u.RawQuery = q.Encode()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	req.Header.Set("PRIVATE-TOKEN", h.cfg.Token)
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	var events []gitlabEvent
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		return nil, err
	}
	out := events[:0]
	for _, e := range events {
		if e.CreatedAt.Before(since) || !e.CreatedAt.Before(until) {
			continue
		}
		if e.ProjectID > 0 {
			path, perr := h.resolveProjectPath(ctx, e.ProjectID)
			if perr == nil {
				e.ProjectFullPath = path
			}
		}
		out = append(out, e)
	}
	return out, nil
}

func (h *HTTPSource) resolveProjectPath(ctx context.Context, id int) (string, error) {
	u := fmt.Sprintf("%s/api/v4/projects/%d", strings.TrimRight(h.cfg.BaseURL, "/"), id)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	req.Header.Set("PRIVATE-TOKEN", h.cfg.Token)
	resp, err := h.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}
	var p struct {
		PathWithNamespace string `json:"path_with_namespace"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return "", err
	}
	return p.PathWithNamespace, nil
}
```

- [ ] **Step 2.4: Create `internal/sync/gitlab/http_test.go`**

```go
package gitlab

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHTTPSource_NotConfigured(t *testing.T) {
	_, err := NewHTTPSource(Config{})
	require.ErrorIs(t, err, ErrNotConfigured)
}

func TestHTTPSource_CommitsBy_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "tok", r.Header.Get("PRIVATE-TOKEN"))
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v4/users") && r.URL.Query().Get("username") == "alice":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id": 42}]`))
		case strings.HasPrefix(r.URL.Path, "/api/v4/users/42/events"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[
				{"created_at":"2026-04-26T10:30:00Z","action_name":"pushed to","project_id":7,
				 "push_data":{"commit_to":"abc1234567def","commit_title":"feat: thing","ref":"main"}}
			]`))
		case strings.HasPrefix(r.URL.Path, "/api/v4/projects/7"):
			_, _ = w.Write([]byte(`{"path_with_namespace":"team/project"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	src, err := NewHTTPSource(Config{BaseURL: srv.URL, Token: "tok"})
	require.NoError(t, err)

	since := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC)
	commits, err := src.CommitsBy(context.Background(), "alice", since, until)
	require.NoError(t, err)
	require.Len(t, commits, 1)
	require.Equal(t, "feat: thing", commits[0].Title)
	require.Equal(t, "team/project", commits[0].Project)
	require.Contains(t, commits[0].URL, "/team/project/-/commit/abc1234567def")
}

func TestHTTPSource_CommitsBy_AuthFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	src, err := NewHTTPSource(Config{BaseURL: srv.URL, Token: "bad"})
	require.NoError(t, err)
	_, err = src.CommitsBy(context.Background(), "alice", time.Now().Add(-24*time.Hour), time.Now())
	require.Error(t, err)
}
```

- [ ] **Step 2.5: Run + commit**

```bash
go test ./internal/sync/gitlab/...
golangci-lint run ./internal/sync/gitlab/...
git add internal/sync/gitlab/
git commit -m "feat: add gitlab source for standup aggregation"
```

Both `TestHTTPSource_*` tests must PASS.

---

## Task 3: Tracker Source — interface, fake, HTTP impl

**Files:**
- Create: `internal/sync/tracker/source.go`
- Create: `internal/sync/tracker/fake.go`
- Create: `internal/sync/tracker/http.go`
- Create: `internal/sync/tracker/http_test.go`

- [ ] **Step 3.1: Create `internal/sync/tracker/source.go`**

```go
// Package tracker is a read-only client for Citeck Project Tracker, used by the
// standup aggregator. No write APIs are exposed.
package tracker

import (
	"context"
	"errors"
	"time"
)

// ErrNotConfigured is returned when Tracker is not configured (missing URL/token/username).
var ErrNotConfigured = errors.New("tracker: not configured")

// Item is a minimal tracker record for standup display.
type Item struct {
	ID     string    // record ref e.g. "task@TICKET-123"
	Key    string    // display key e.g. "TICKET-123"
	Title  string
	Status string    // current status
	URL    string    // https://tracker/.../v_app/task@TICKET-123
	At     time.Time // last-modified
}

// Source fetches tracker items recently active for the configured user.
type Source interface {
	// AssignedActive returns items assigned to `username` whose modifiedAt falls in
	// [since, until). Sorted DESC by At.
	AssignedActive(ctx context.Context, username string, since, until time.Time) ([]Item, error)
}
```

- [ ] **Step 3.2: Create `internal/sync/tracker/fake.go`**

```go
package tracker

import (
	"context"
	"sort"
	"sync"
	"time"
)

// Fake is an in-memory Source for tests.
type Fake struct {
	mu    sync.Mutex
	items []Item
	err   error
}

// NewFake returns an empty Fake.
func NewFake() *Fake { return &Fake{} }

// SetItems replaces the canned item list.
func (f *Fake) SetItems(it []Item) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.items = append([]Item(nil), it...)
}

// SetError sets a sticky error returned by every AssignedActive call.
func (f *Fake) SetError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

// AssignedActive filters the canned list by range.
func (f *Fake) AssignedActive(_ context.Context, username string, since, until time.Time) ([]Item, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	_ = username
	out := make([]Item, 0, len(f.items))
	for _, it := range f.items {
		if (it.At.Equal(since) || it.At.After(since)) && it.At.Before(until) {
			out = append(out, it)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At.After(out[j].At) })
	return out, nil
}
```

- [ ] **Step 3.3: Create `internal/sync/tracker/http.go`**

```go
package tracker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Config configures a HTTPSource.
type Config struct {
	BaseURL  string // e.g. https://tracker.example.com
	Username string // basic-auth username (or service account name)
	Token    string // password / API token
	Timeout  time.Duration
}

// HTTPSource calls Citeck PT records query API to fetch user-assigned items.
type HTTPSource struct {
	cfg    Config
	client *http.Client
}

// NewHTTPSource constructs a HTTPSource. Returns ErrNotConfigured if any of BaseURL/Username/Token is empty.
func NewHTTPSource(cfg Config) (*HTTPSource, error) {
	if strings.TrimSpace(cfg.BaseURL) == "" || strings.TrimSpace(cfg.Username) == "" || strings.TrimSpace(cfg.Token) == "" {
		return nil, ErrNotConfigured
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 15 * time.Second
	}
	return &HTTPSource{cfg: cfg, client: &http.Client{Timeout: cfg.Timeout}}, nil
}

type queryRequest struct {
	Query    queryBody `json:"query"`
	Language string    `json:"language"`
	Page     pageBody  `json:"page"`
}

type queryBody struct {
	SourceID  string         `json:"sourceId"`
	Query     map[string]any `json:"query"`
	GroupBy   []string       `json:"groupBy,omitempty"`
	Consistency string       `json:"consistency,omitempty"`
}

type pageBody struct {
	MaxItems int `json:"maxItems"`
	Skip     int `json:"skip"`
}

type queryResponse struct {
	Records []struct {
		ID         string `json:"id"`
		Attributes struct {
			DispName     string `json:"_disp"`
			Status       string `json:"_status"`
			ModifiedAt   string `json:"_modified"`
		} `json:"attributes"`
	} `json:"records"`
}

// AssignedActive queries PT for tasks where assignee=username and modifiedAt in [since, until).
func (h *HTTPSource) AssignedActive(ctx context.Context, username string, since, until time.Time) ([]Item, error) {
	body := queryRequest{
		Query: queryBody{
			SourceID: "emodel/task",
			Query: map[string]any{
				"assignee":   username,
				"_modified>": since.UTC().Format(time.RFC3339),
				"_modified<": until.UTC().Format(time.RFC3339),
			},
			Consistency: "EVENTUAL",
		},
		Language: "predicate",
		Page:     pageBody{MaxItems: 100},
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	endpoint := strings.TrimRight(h.cfg.BaseURL, "/") + "/gateway/emodel/api/records/query"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(h.cfg.Username, h.cfg.Token)

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var qr queryResponse
	if err := json.NewDecoder(resp.Body).Decode(&qr); err != nil {
		return nil, err
	}

	out := make([]Item, 0, len(qr.Records))
	for _, r := range qr.Records {
		at, _ := time.Parse(time.RFC3339, r.Attributes.ModifiedAt)
		key := r.ID
		if i := strings.LastIndex(r.ID, "@"); i >= 0 && i+1 < len(r.ID) {
			key = r.ID[i+1:]
		}
		out = append(out, Item{
			ID:     r.ID,
			Key:    key,
			Title:  r.Attributes.DispName,
			Status: r.Attributes.Status,
			URL:    strings.TrimRight(h.cfg.BaseURL, "/") + "/v2/dashboard?recordRef=" + r.ID,
			At:     at,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At.After(out[j].At) })
	return out, nil
}
```

- [ ] **Step 3.4: Create `internal/sync/tracker/http_test.go`**

```go
package tracker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHTTPSource_NotConfigured(t *testing.T) {
	_, err := NewHTTPSource(Config{BaseURL: "x"})
	require.ErrorIs(t, err, ErrNotConfigured)
}

func TestHTTPSource_AssignedActive_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/gateway/emodel/api/records/query", r.URL.Path)
		u, p, ok := r.BasicAuth()
		require.True(t, ok)
		require.Equal(t, "alice", u)
		require.Equal(t, "tok", p)

		var body queryRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, "emodel/task", body.Query.SourceID)
		require.Equal(t, "alice", body.Query.Query["assignee"])

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"records":[
			{"id":"emodel/task@TICKET-1","attributes":{"_disp":"Title 1","_status":"in_progress","_modified":"2026-04-26T12:00:00Z"}},
			{"id":"emodel/task@TICKET-2","attributes":{"_disp":"Title 2","_status":"done","_modified":"2026-04-26T18:00:00Z"}}
		]}`))
	}))
	defer srv.Close()

	src, err := NewHTTPSource(Config{BaseURL: srv.URL, Username: "alice", Token: "tok"})
	require.NoError(t, err)
	since := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC)
	items, err := src.AssignedActive(context.Background(), "alice", since, until)
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, "TICKET-2", items[0].Key)
	require.Equal(t, "done", items[0].Status)
	require.Contains(t, items[0].URL, "recordRef=emodel/task@TICKET-2")
}
```

- [ ] **Step 3.5: Run + commit**

```bash
go test ./internal/sync/tracker/...
golangci-lint run ./internal/sync/tracker/...
git add internal/sync/tracker/
git commit -m "feat: add tracker source for standup aggregation"
```

Both `TestHTTPSource_*` tests must PASS.

---

## Task 4: Standup domain — service + tests

**Files:**
- Create: `internal/standup/service.go`
- Create: `internal/standup/service_test.go`

- [ ] **Step 4.1: Create `internal/standup/service.go`**

```go
// Package standup aggregates "Yesterday / Today / Blockers" from closed todos,
// GitLab commits, and Citeck Project Tracker activity.
package standup

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/clock"
	"github.com/spk/spk-cockpit/internal/sync/gitlab"
	"github.com/spk/spk-cockpit/internal/sync/tracker"
	"github.com/spk/spk-cockpit/internal/todo"
)

// TodoQuerier is the subset of todo.Service that standup needs.
type TodoQuerier interface {
	List(ctx context.Context, f todo.TodoFilter) ([]api.Todo, error)
	History(ctx context.Context, id string, limit int) ([]api.TodoEvent, error)
}

// EventLister yields all todo audit events since a given unix-second timestamp.
type EventLister interface {
	ListAll(ctx context.Context, sinceUnix int64, limit int) ([]api.TodoEvent, error)
}

// Config wires the dependencies for a Service.
type Config struct {
	Todos        TodoQuerier
	Events       EventLister
	GitLab       gitlab.Source // nil if disabled
	Tracker      tracker.Source // nil if disabled
	GitLabAuthor string         // empty disables fan-out
	TrackerUser  string         // empty disables fan-out
	Clock        clock.Clock
}

// Service is the standup aggregator. Stateless; safe for concurrent calls.
type Service struct {
	cfg Config
}

// NewService constructs a Service.
func NewService(cfg Config) *Service { return &Service{cfg: cfg} }

// Generate builds the report for the local-day `day` (its date is used; time-of-day ignored).
//
// "Yesterday" = items active in [day-1d 00:00, day 00:00) local time.
// "Today"     = open in_progress todos + open todos with due_at within [day 00:00, day+1d) local time.
// "Blockers"  = urgent/high open todos with due_at < day 00:00 local time.
func (s *Service) Generate(ctx context.Context, day time.Time) (api.StandupReport, error) {
	loc := day.Location()
	dayStart := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc)
	dayEnd := dayStart.Add(24 * time.Hour)
	yStart := dayStart.Add(-24 * time.Hour)

	report := api.StandupReport{
		Day:       dayStart.Format("2006-01-02"),
		Yesterday: []api.StandupItem{},
		Today:     []api.StandupItem{},
		Blockers:  []api.StandupItem{},
	}

	var (
		mu      sync.Mutex
		errs    []string
		wg      sync.WaitGroup
	)
	addErr := func(label string, err error) {
		mu.Lock()
		errs = append(errs, label+": "+err.Error())
		mu.Unlock()
	}
	addItems := func(section api.StandupSection, items []api.StandupItem) {
		mu.Lock()
		defer mu.Unlock()
		switch section {
		case api.StandupSectionYesterday:
			report.Yesterday = append(report.Yesterday, items...)
		case api.StandupSectionToday:
			report.Today = append(report.Today, items...)
		case api.StandupSectionBlockers:
			report.Blockers = append(report.Blockers, items...)
		}
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		yest, today, blockers, err := s.todoBuckets(ctx, yStart, dayStart, dayEnd)
		if err != nil {
			addErr("todos", err)
			return
		}
		addItems(api.StandupSectionYesterday, yest)
		addItems(api.StandupSectionToday, today)
		addItems(api.StandupSectionBlockers, blockers)
	}()

	if s.cfg.GitLab != nil && s.cfg.GitLabAuthor != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			items, err := s.gitlabBucket(ctx, yStart, dayStart)
			if err != nil {
				addErr("gitlab", err)
				return
			}
			addItems(api.StandupSectionYesterday, items)
		}()
	}

	if s.cfg.Tracker != nil && s.cfg.TrackerUser != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			items, err := s.trackerBucket(ctx, yStart, dayStart)
			if err != nil {
				addErr("tracker", err)
				return
			}
			addItems(api.StandupSectionYesterday, items)
		}()
	}

	wg.Wait()

	sortByAtDesc(report.Yesterday)
	sortByAtDesc(report.Today)
	sortByAtDesc(report.Blockers)

	if len(errs) > 0 {
		report.Errors = errs
		sort.Strings(report.Errors)
	}
	return report, nil
}

func (s *Service) todoBuckets(ctx context.Context, yStart, dayStart, dayEnd time.Time) (yest, today, blockers []api.StandupItem, err error) {
	if s.cfg.Events == nil || s.cfg.Todos == nil {
		return nil, nil, nil, errors.New("todos: not configured")
	}
	events, err := s.cfg.Events.ListAll(ctx, yStart.Unix(), 500)
	if err != nil {
		return nil, nil, nil, err
	}
	seen := make(map[string]struct{})
	for _, e := range events {
		if e.Kind != "status_changed" || e.ToValue != string(api.StatusDone) {
			continue
		}
		if e.At < yStart.Unix() || e.At >= dayStart.Unix() {
			continue
		}
		if _, ok := seen[e.TodoID]; ok {
			continue
		}
		seen[e.TodoID] = struct{}{}
		yest = append(yest, api.StandupItem{
			Source: api.StandupSourceTodo,
			Title:  todoTitle(ctx, s.cfg.Todos, e.TodoID),
			Detail: "done",
			RefID:  e.TodoID,
			At:     e.At,
		})
	}

	open, err := s.cfg.Todos.List(ctx, todo.TodoFilter{Statuses: []api.TodoStatus{api.StatusInProgress, api.StatusOpen}})
	if err != nil {
		return nil, nil, nil, err
	}
	for _, t := range open {
		if t.Status == api.StatusInProgress {
			today = append(today, api.StandupItem{
				Source: api.StandupSourceTodo,
				Title:  t.Title,
				Detail: "in progress",
				RefID:  t.ID,
				At:     t.UpdatedAt,
			})
			continue
		}
		if t.DueAt != nil && *t.DueAt >= dayStart.Unix() && *t.DueAt < dayEnd.Unix() {
			today = append(today, api.StandupItem{
				Source: api.StandupSourceTodo,
				Title:  t.Title,
				Detail: "due today",
				RefID:  t.ID,
				At:     *t.DueAt,
			})
			continue
		}
		if t.DueAt != nil && *t.DueAt < dayStart.Unix() && (t.Priority == api.PriorityUrgent || t.Priority == api.PriorityHigh) {
			blockers = append(blockers, api.StandupItem{
				Source: api.StandupSourceTodo,
				Title:  t.Title,
				Detail: "overdue",
				RefID:  t.ID,
				At:     *t.DueAt,
			})
		}
	}
	return yest, today, blockers, nil
}

func (s *Service) gitlabBucket(ctx context.Context, since, until time.Time) ([]api.StandupItem, error) {
	commits, err := s.cfg.GitLab.CommitsBy(ctx, s.cfg.GitLabAuthor, since, until)
	if err != nil {
		return nil, err
	}
	out := make([]api.StandupItem, 0, len(commits))
	for _, c := range commits {
		out = append(out, api.StandupItem{
			Source: api.StandupSourceGitLab,
			Title:  c.Title,
			Detail: c.Project,
			URL:    c.URL,
			RefID:  c.SHA,
			At:     c.At.Unix(),
		})
	}
	return out, nil
}

func (s *Service) trackerBucket(ctx context.Context, since, until time.Time) ([]api.StandupItem, error) {
	items, err := s.cfg.Tracker.AssignedActive(ctx, s.cfg.TrackerUser, since, until)
	if err != nil {
		return nil, err
	}
	out := make([]api.StandupItem, 0, len(items))
	for _, it := range items {
		out = append(out, api.StandupItem{
			Source: api.StandupSourceTracker,
			Title:  it.Key + ": " + it.Title,
			Detail: it.Status,
			URL:    it.URL,
			RefID:  it.ID,
			At:     it.At.Unix(),
		})
	}
	return out, nil
}

func todoTitle(ctx context.Context, q TodoQuerier, id string) string {
	list, err := q.List(ctx, todo.TodoFilter{IncludeDone: true})
	if err != nil {
		return id
	}
	for _, t := range list {
		if t.ID == id {
			return t.Title
		}
	}
	return id
}

func sortByAtDesc(items []api.StandupItem) {
	sort.SliceStable(items, func(i, j int) bool { return items[i].At > items[j].At })
}
```

- [ ] **Step 4.2: Create `internal/standup/service_test.go`**

```go
package standup_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/clock"
	"github.com/spk/spk-cockpit/internal/eventbus"
	"github.com/spk/spk-cockpit/internal/standup"
	"github.com/spk/spk-cockpit/internal/sync/gitlab"
	"github.com/spk/spk-cockpit/internal/sync/tracker"
	"github.com/spk/spk-cockpit/internal/todo"
	"github.com/spk/spk-cockpit/internal/todo/fakerepo"
)

type stubEvents struct {
	events []api.TodoEvent
	err    error
}

func (s *stubEvents) ListAll(_ context.Context, _ int64, _ int) ([]api.TodoEvent, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.events, nil
}

func (s *stubEvents) Append(_ context.Context, e api.TodoEvent) error {
	s.events = append(s.events, e)
	return nil
}

func (s *stubEvents) ListByTodo(_ context.Context, _ string, _ int) ([]api.TodoEvent, error) {
	return nil, nil
}

func newTodoSvc(t *testing.T) (*todo.Service, *stubEvents) {
	t.Helper()
	repo := fakerepo.NewTodo()
	tags := fakerepo.NewTag()
	events := &stubEvents{}
	bus := eventbus.New(8)
	t.Cleanup(func() { bus.Close() })
	svc := todo.NewService(repo, tags, events, clock.Real(), bus)
	return svc, events
}

func TestService_Generate_TodosOnly(t *testing.T) {
	day := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	yStart := day.Truncate(24 * time.Hour).Add(-24 * time.Hour)

	svc, events := newTodoSvc(t)
	ctx := context.Background()

	// Yesterday: a todo that was completed yesterday.
	created, err := svc.Create(ctx, api.CreateTodoRequest{Title: "Ship feature X"})
	require.NoError(t, err)
	doneStatus := api.StatusDone
	_, err = svc.Update(ctx, created.ID, api.UpdateTodoRequest{Status: &doneStatus})
	require.NoError(t, err)

	// Override the event time to land in yesterday's window.
	for i := range events.events {
		if events.events[i].Kind == "status_changed" && events.events[i].ToValue == string(api.StatusDone) {
			events.events[i].At = yStart.Add(8 * time.Hour).Unix()
		}
	}

	// Today (in-progress).
	wip, err := svc.Create(ctx, api.CreateTodoRequest{Title: "Polish settings UI"})
	require.NoError(t, err)
	wipStatus := api.StatusInProgress
	_, err = svc.Update(ctx, wip.ID, api.UpdateTodoRequest{Status: &wipStatus})
	require.NoError(t, err)

	// Blocker: urgent overdue.
	dueOverdue := yStart.Add(-48 * time.Hour).Unix()
	urgent := api.PriorityUrgent
	_, err = svc.Create(ctx, api.CreateTodoRequest{Title: "Fix prod crash", Priority: &urgent, DueAt: &dueOverdue})
	require.NoError(t, err)

	s := standup.NewService(standup.Config{
		Todos:  svc,
		Events: events,
		Clock:  clock.Real(),
	})
	report, err := s.Generate(ctx, day)
	require.NoError(t, err)

	require.Equal(t, "2026-04-27", report.Day)
	require.Len(t, report.Yesterday, 1)
	require.Equal(t, "Ship feature X", report.Yesterday[0].Title)
	require.Equal(t, api.StandupSourceTodo, report.Yesterday[0].Source)

	require.GreaterOrEqual(t, len(report.Today), 1)
	foundWIP := false
	for _, it := range report.Today {
		if it.Title == "Polish settings UI" && it.Detail == "in progress" {
			foundWIP = true
		}
	}
	require.True(t, foundWIP, "expected WIP todo in Today")

	require.Len(t, report.Blockers, 1)
	require.Equal(t, "Fix prod crash", report.Blockers[0].Title)
	require.Equal(t, "overdue", report.Blockers[0].Detail)
}

func TestService_Generate_GitLabAndTrackerErrorsCollected(t *testing.T) {
	day := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	svc, events := newTodoSvc(t)

	gl := gitlab.NewFake()
	gl.SetError(errors.New("401"))
	tr := tracker.NewFake()
	tr.SetError(errors.New("500"))

	s := standup.NewService(standup.Config{
		Todos:        svc,
		Events:       events,
		GitLab:       gl,
		GitLabAuthor: "alice",
		Tracker:      tr,
		TrackerUser:  "alice",
		Clock:        clock.Real(),
	})
	report, err := s.Generate(context.Background(), day)
	require.NoError(t, err)
	require.Len(t, report.Errors, 2)
	require.Contains(t, report.Errors[0]+report.Errors[1], "gitlab")
	require.Contains(t, report.Errors[0]+report.Errors[1], "tracker")
}

func TestService_Generate_GitLabCommitsAppearInYesterday(t *testing.T) {
	day := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	svc, events := newTodoSvc(t)
	yStart := day.Truncate(24 * time.Hour).Add(-24 * time.Hour)

	gl := gitlab.NewFake()
	gl.SetCommits([]gitlab.Commit{{
		SHA: "abc123", Title: "feat: thing", Project: "team/x",
		URL: "https://gl/team/x/-/commit/abc123",
		At:  yStart.Add(10 * time.Hour),
	}})

	s := standup.NewService(standup.Config{
		Todos:        svc,
		Events:       events,
		GitLab:       gl,
		GitLabAuthor: "alice",
		Clock:        clock.Real(),
	})
	report, err := s.Generate(context.Background(), day)
	require.NoError(t, err)
	require.Len(t, report.Yesterday, 1)
	require.Equal(t, api.StandupSourceGitLab, report.Yesterday[0].Source)
	require.Equal(t, "feat: thing", report.Yesterday[0].Title)
}
```

- [ ] **Step 4.3: Run + commit**

```bash
go test ./internal/standup/...
golangci-lint run ./internal/standup/...
git add internal/standup/
git commit -m "feat: add standup aggregator service"
```

All `TestService_Generate_*` tests must PASS.

---

## Task 5: Standup markdown formatter

**Files:**
- Create: `internal/standup/markdown.go`
- Create: `internal/standup/markdown_test.go`

- [ ] **Step 5.1: Create `internal/standup/markdown.go`**

```go
package standup

import (
	"fmt"
	"strings"

	"github.com/spk/spk-cockpit/internal/api"
)

// Markdown renders a StandupReport as a copy-paste-friendly markdown block.
// The format is:
//
//	# Standup — YYYY-MM-DD
//
//	## Yesterday
//	- [todo] Title — done
//	- [git] feat: thing — team/x
//	- [pt] TICKET-1: Title — done
//
//	## Today
//	- [todo] Title — in progress
//
//	## Blockers
//	- [todo] Title — overdue
//
// Empty sections are still emitted with "_(none)_" so the structure stays predictable.
func Markdown(r api.StandupReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Standup — %s\n\n", r.Day)
	writeSection(&b, "Yesterday", r.Yesterday)
	writeSection(&b, "Today", r.Today)
	writeSection(&b, "Blockers", r.Blockers)
	if len(r.Errors) > 0 {
		b.WriteString("\n_Source errors:_\n")
		for _, e := range r.Errors {
			fmt.Fprintf(&b, "- %s\n", e)
		}
	}
	return b.String()
}

func writeSection(b *strings.Builder, name string, items []api.StandupItem) {
	fmt.Fprintf(b, "## %s\n", name)
	if len(items) == 0 {
		b.WriteString("_(none)_\n\n")
		return
	}
	for _, it := range items {
		tag := tagFor(it.Source)
		line := fmt.Sprintf("- %s %s", tag, it.Title)
		if it.Detail != "" {
			line += " — " + it.Detail
		}
		if it.URL != "" {
			line += " (" + it.URL + ")"
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	b.WriteString("\n")
}

func tagFor(s api.StandupItemSource) string {
	switch s {
	case api.StandupSourceTodo:
		return "[todo]"
	case api.StandupSourceGitLab:
		return "[git]"
	case api.StandupSourceTracker:
		return "[pt]"
	default:
		return "[?]"
	}
}
```

- [ ] **Step 5.2: Create `internal/standup/markdown_test.go`**

```go
package standup_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/standup"
)

func TestMarkdown_FullReport(t *testing.T) {
	r := api.StandupReport{
		Day: "2026-04-27",
		Yesterday: []api.StandupItem{
			{Source: api.StandupSourceTodo, Title: "Ship X", Detail: "done"},
			{Source: api.StandupSourceGitLab, Title: "feat: thing", Detail: "team/x", URL: "https://gl/x"},
		},
		Today: []api.StandupItem{
			{Source: api.StandupSourceTodo, Title: "Polish UI", Detail: "in progress"},
		},
		Blockers: nil,
	}
	md := standup.Markdown(r)
	require.True(t, strings.HasPrefix(md, "# Standup — 2026-04-27"))
	require.Contains(t, md, "## Yesterday\n")
	require.Contains(t, md, "- [todo] Ship X — done\n")
	require.Contains(t, md, "- [git] feat: thing — team/x (https://gl/x)\n")
	require.Contains(t, md, "## Today\n")
	require.Contains(t, md, "- [todo] Polish UI — in progress\n")
	require.Contains(t, md, "## Blockers\n_(none)_\n")
}

func TestMarkdown_WithErrors(t *testing.T) {
	r := api.StandupReport{
		Day:    "2026-04-27",
		Errors: []string{"gitlab: 401"},
	}
	md := standup.Markdown(r)
	require.Contains(t, md, "_Source errors:_")
	require.Contains(t, md, "- gitlab: 401")
}
```

- [ ] **Step 5.3: Run + commit**

```bash
go test ./internal/standup/...
golangci-lint run ./internal/standup/...
git add internal/standup/markdown.go internal/standup/markdown_test.go
git commit -m "feat: add standup markdown formatter"
```

Both `TestMarkdown_*` tests must PASS.

---

## Task 6: Server handler — `GET /api/standup`

**Files:**
- Modify: `internal/server/server.go`
- Modify: `internal/server/routes.go`
- Create: `internal/server/standup_handler.go`
- Create: `internal/server/standup_handler_test.go`

- [ ] **Step 6.1: Modify `internal/server/server.go`**

Add `Standup *standup.Service` to `Deps` (near other services). The full Deps block becomes:

```go
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
```

Add the import:

```go
"github.com/spk/spk-cockpit/internal/standup"
```

- [ ] **Step 6.2: Add route in `internal/server/routes.go`**

After the existing `mux.HandleFunc("GET /api/sync", handleSyncStatus(d))` line, add:

```go
mux.HandleFunc("GET /api/standup", handleStandup(d))
```

- [ ] **Step 6.3: Create `internal/server/standup_handler.go`**

```go
package server

import (
	"encoding/json"
	"net/http"
	"time"
)

// handleStandup serves GET /api/standup?date=YYYY-MM-DD.
// If date is omitted, today's local date is used.
func handleStandup(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if d.Standup == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "standup.unavailable", "standup service not configured")
			return
		}
		day := time.Now()
		if q := r.URL.Query().Get("date"); q != "" {
			parsed, err := time.ParseInLocation("2006-01-02", q, time.Local)
			if err != nil {
				writeAPIError(w, http.StatusBadRequest, "standup.bad_date", "invalid date (expected YYYY-MM-DD)")
				return
			}
			day = parsed
		}
		report, err := d.Standup.Generate(r.Context(), day)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "standup.generate_failed", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(report)
	}
}
```

If `writeAPIError` is not yet a shared helper in `server`, check `internal/server/` for the existing error-writing pattern (look at `meeting_handler.go`) and reuse it. If it's named differently (e.g., `respondError`), use that name instead — keep this handler consistent.

- [ ] **Step 6.4: Create `internal/server/standup_handler_test.go`**

```go
package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/clock"
	"github.com/spk/spk-cockpit/internal/eventbus"
	"github.com/spk/spk-cockpit/internal/standup"
	"github.com/spk/spk-cockpit/internal/todo"
	"github.com/spk/spk-cockpit/internal/todo/fakerepo"
)

type fakeEvents struct{}

func (fakeEvents) ListAll(_ context.Context, _ int64, _ int) ([]api.TodoEvent, error) {
	return nil, nil
}
func (fakeEvents) Append(_ context.Context, _ api.TodoEvent) error              { return nil }
func (fakeEvents) ListByTodo(_ context.Context, _ string, _ int) ([]api.TodoEvent, error) {
	return nil, nil
}

func TestStandupHandler_Empty(t *testing.T) {
	bus := eventbus.New(4)
	defer bus.Close()
	svc := todo.NewService(fakerepo.NewTodo(), fakerepo.NewTag(), fakeEvents{}, clock.Real(), bus)
	st := standup.NewService(standup.Config{Todos: svc, Events: fakeEvents{}, Clock: clock.Real()})

	deps := &Deps{Standup: st}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/standup", handleStandup(deps))

	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/standup?date=2026-04-27")
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var got api.StandupReport
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	require.Equal(t, "2026-04-27", got.Day)
	require.NotNil(t, got.Yesterday)
}

func TestStandupHandler_BadDate(t *testing.T) {
	deps := &Deps{Standup: standup.NewService(standup.Config{Clock: clock.Real()})}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/standup", handleStandup(deps))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/standup?date=oops")
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestStandupHandler_Unconfigured(t *testing.T) {
	deps := &Deps{} // Standup is nil
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/standup", handleStandup(deps))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/standup")
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	_ = time.Now()
}
```

- [ ] **Step 6.5: Run + commit**

```bash
go test ./internal/server/...
golangci-lint run ./internal/server/...
git add internal/server/server.go internal/server/routes.go internal/server/standup_handler.go internal/server/standup_handler_test.go
git commit -m "feat: add /api/standup handler"
```

All three `TestStandupHandler_*` tests must PASS.

---

## Task 7: Wire standup + GitLab + Tracker in `cli/start.go`

**Files:**
- Modify: `internal/cli/start.go`

- [ ] **Step 7.1: Add imports**

In the import block of `internal/cli/start.go`, add:

```go
"github.com/spk/spk-cockpit/internal/standup"
"github.com/spk/spk-cockpit/internal/sync/gitlab"
"github.com/spk/spk-cockpit/internal/sync/tracker"
```

- [ ] **Step 7.2: Build sources after `meetingSvc`/`secretSvc` are created**

After the existing `srv.Deps().Kv = store.NewKvRepo(st.DB)` line, add:

```go
kv := store.NewKvRepo(st.DB)
gitlabSrc := buildGitLabSource(ctx, kv, secretSvc, logger)
trackerSrc := buildTrackerSource(ctx, kv, secretSvc, logger)
glAuthor, _, _ := kv.Get(ctx, "gitlab.author_username")
trackerUser, _, _ := kv.Get(ctx, "tracker.username")

eventRepoIface := eventRepo // existing variable from earlier block
standupSvc := standup.NewService(standup.Config{
	Todos:        todoSvc,
	Events:       eventRepoIface,
	GitLab:       gitlabSrc,
	GitLabAuthor: glAuthor,
	Tracker:      trackerSrc,
	TrackerUser:  trackerUser,
	Clock:        clock.Real(),
})
srv.Deps().Standup = standupSvc
```

If `eventRepo` does not exist yet in `start.go`'s scope, declare it where `todoRepo` is created:

```go
eventRepo := store.NewEventRepo(st.DB)
todoSvc := todo.NewService(todoRepo, tagRepo, eventRepo, clock.Real(), bus)
```

(it should already exist — see existing code at line ~81; verify and reuse.)

- [ ] **Step 7.3: Add helper functions at the bottom of `start.go`**

```go
func buildGitLabSource(ctx context.Context, kv todo.KvRepo, secrets *secret.Service, logger *slog.Logger) gitlab.Source {
	url, _, _ := kv.Get(ctx, "gitlab.url")
	if url == "" {
		return nil
	}
	tok, err := secrets.Get(ctx, "gitlab_token")
	if err != nil || tok == "" {
		return nil
	}
	src, err := gitlab.NewHTTPSource(gitlab.Config{BaseURL: url, Token: tok})
	if err != nil {
		logger.Warn("gitlab source disabled", "err", err)
		return nil
	}
	return src
}

func buildTrackerSource(ctx context.Context, kv todo.KvRepo, secrets *secret.Service, logger *slog.Logger) tracker.Source {
	url, _, _ := kv.Get(ctx, "tracker.url")
	user, _, _ := kv.Get(ctx, "tracker.username")
	if url == "" || user == "" {
		return nil
	}
	tok, err := secrets.Get(ctx, "tracker_token")
	if err != nil || tok == "" {
		return nil
	}
	src, err := tracker.NewHTTPSource(tracker.Config{BaseURL: url, Username: user, Token: tok})
	if err != nil {
		logger.Warn("tracker source disabled", "err", err)
		return nil
	}
	return src
}
```

Add `"log/slog"` and `"github.com/spk/spk-cockpit/internal/todo"` to imports if not already present.

- [ ] **Step 7.4: Run + commit**

```bash
go build ./...
go test ./internal/cli/...
golangci-lint run ./internal/cli/...
git add internal/cli/start.go
git commit -m "feat: wire standup service + gitlab/tracker sources in start.go"
```

`go build` must produce no errors; existing `internal/cli` tests must still PASS.

---

## Task 8: CLI `cockpit standup` subcommand

**Files:**
- Modify: `internal/cli/client.go`
- Create: `internal/cli/standup.go`

- [ ] **Step 8.1: Add `Standup` method to `internal/cli/client.go`**

Append, before `ErrDaemonNotRunning`:

```go
// Standup fetches the standup report for the given date (YYYY-MM-DD or empty for today).
func (c *Client) Standup(ctx context.Context, date string) (api.StandupReport, error) {
	path := "/api/standup"
	if date != "" {
		path += "?date=" + date
	}
	var out api.StandupReport
	if err := c.getJSON(ctx, path, &out); err != nil {
		return api.StandupReport{}, err
	}
	return out, nil
}
```

- [ ] **Step 8.2: Create `internal/cli/standup.go`**

```go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/spk/spk-cockpit/internal/paths"
	"github.com/spk/spk-cockpit/internal/standup"
)

var standupFlags struct {
	date string
}

var standupCmd = &cobra.Command{
	Use:   "standup",
	Short: "Print today's standup as markdown (yesterday/today/blockers)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		p, err := paths.New()
		if err != nil {
			return err
		}
		c := NewClient(p.SocketFile)
		report, err := c.Standup(cmd.Context(), standupFlags.date)
		if err != nil {
			return err
		}
		fmt.Print(standup.Markdown(report))
		return nil
	},
}

func init() {
	standupCmd.Flags().StringVar(&standupFlags.date, "date", "", "Day to report on (YYYY-MM-DD), default today")
	rootCmd.AddCommand(standupCmd)
}
```

- [ ] **Step 8.3: Manual smoke (no automated test for this trivial wiring)**

```bash
go build -o /tmp/cockpit-build-check ./cmd/cockpit
/tmp/cockpit-build-check standup --help
```

Expected: command help is shown, no panic.

- [ ] **Step 8.4: Commit**

```bash
go vet ./...
golangci-lint run ./internal/cli/...
git add internal/cli/client.go internal/cli/standup.go
git commit -m "feat: add cockpit standup CLI subcommand"
```

---

## Task 9: Web `/standup` page + Copy as markdown

**Files:**
- Modify: `web/src/lib/types.ts`
- Modify: `web/src/lib/api.ts`
- Modify: `web/src/App.tsx`
- Create: `web/src/pages/Standup.tsx`

- [ ] **Step 9.1: Append types in `web/src/lib/types.ts`**

```ts
export type StandupItemSource = "todo" | "gitlab" | "tracker";

export interface StandupItem {
  source: StandupItemSource;
  title: string;
  detail?: string;
  url?: string;
  refId?: string;
  at: number;
}

export interface StandupReport {
  day: string;
  yesterday: StandupItem[];
  today: StandupItem[];
  blockers: StandupItem[];
  errors?: string[];
}
```

- [ ] **Step 9.2: Append API method in `web/src/lib/api.ts`**

Inside the existing API object (or as a free function — match the pattern already used for `fetchMeetings`), add:

```ts
export async function fetchStandup(date?: string): Promise<StandupReport> {
  const q = date ? `?date=${encodeURIComponent(date)}` : "";
  const resp = await fetch(`/api/standup${q}`);
  if (!resp.ok) {
    throw new Error(`standup: ${resp.status}`);
  }
  return (await resp.json()) as StandupReport;
}
```

Add the import for `StandupReport` at the top: `import type { StandupReport } from "./types";` (or extend the existing types import).

- [ ] **Step 9.3: Wire `/standup` route in `web/src/App.tsx`**

Add the import:

```tsx
import { Standup } from "./pages/Standup";
```

Add a nav link inside `MainShell`'s `nav`:

```tsx
{navItem("/standup", "Standup")}
```

(insert it after `Calendar` and before `Settings` so the order is Todos → Calendar → Standup → Settings).

Add a route inside `MainShell`'s inner `<Routes>`:

```tsx
<Route path="/standup" element={<Standup />} />
```

(insert before the catch-all `<Route path="*" .../>`.)

- [ ] **Step 9.4: Create `web/src/pages/Standup.tsx`**

```tsx
import { useEffect, useState } from "react";
import type { StandupReport, StandupItem } from "../lib/types";
import { fetchStandup } from "../lib/api";

export function Standup() {
  const [report, setReport] = useState<StandupReport | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    let cancelled = false;
    fetchStandup()
      .then((r) => {
        if (!cancelled) setReport(r);
      })
      .catch((e) => {
        if (!cancelled) setError(String(e));
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const copy = async () => {
    if (!report) return;
    await navigator.clipboard.writeText(toMarkdown(report));
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  if (error) return <div className="text-red-400">Failed to load standup: {error}</div>;
  if (!report) return <div className="text-fgmute">Loading…</div>;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-semibold">Standup — {report.day}</h2>
        <button
          onClick={copy}
          className="px-3 py-1 rounded bg-bgsub border border-bgmute hover:bg-bg text-sm"
        >
          {copied ? "Copied!" : "Copy as markdown"}
        </button>
      </div>
      {report.errors && report.errors.length > 0 && (
        <div className="text-yellow-400 text-sm">
          {report.errors.map((e, i) => (
            <div key={i}>⚠ {e}</div>
          ))}
        </div>
      )}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Section title="Yesterday" items={report.yesterday} />
        <Section title="Today" items={report.today} />
        <Section title="Blockers" items={report.blockers} />
      </div>
    </div>
  );
}

function Section({ title, items }: { title: string; items: StandupItem[] }) {
  return (
    <div className="bg-bgsub border border-bgmute rounded p-3">
      <h3 className="text-sm uppercase tracking-wide text-fgmute mb-2">{title}</h3>
      {items.length === 0 ? (
        <div className="text-fgmute text-sm">— nothing —</div>
      ) : (
        <ul className="space-y-2">
          {items.map((it, i) => (
            <li key={`${it.refId}-${i}`} className="text-sm">
              <span className="text-fgmute mr-2">[{tagFor(it.source)}]</span>
              {it.url ? (
                <a className="hover:underline" href={it.url} target="_blank" rel="noreferrer">
                  {it.title}
                </a>
              ) : (
                it.title
              )}
              {it.detail && <span className="text-fgmute"> — {it.detail}</span>}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function tagFor(s: StandupItem["source"]): string {
  switch (s) {
    case "todo":
      return "todo";
    case "gitlab":
      return "git";
    case "tracker":
      return "pt";
  }
}

function toMarkdown(r: StandupReport): string {
  let s = `# Standup — ${r.day}\n\n`;
  s += sectionMd("Yesterday", r.yesterday);
  s += sectionMd("Today", r.today);
  s += sectionMd("Blockers", r.blockers);
  if (r.errors && r.errors.length > 0) {
    s += "\n_Source errors:_\n";
    for (const e of r.errors) s += `- ${e}\n`;
  }
  return s;
}

function sectionMd(name: string, items: StandupItem[]): string {
  let s = `## ${name}\n`;
  if (items.length === 0) {
    s += "_(none)_\n\n";
    return s;
  }
  for (const it of items) {
    let line = `- [${tagFor(it.source)}] ${it.title}`;
    if (it.detail) line += ` — ${it.detail}`;
    if (it.url) line += ` (${it.url})`;
    s += line + "\n";
  }
  return s + "\n";
}
```

- [ ] **Step 9.5: Verify web build**

```bash
cd web && pnpm build && cd ..
```

Expected: `web/dist/` is rebuilt with no errors.

- [ ] **Step 9.6: Commit**

```bash
git add web/src/lib/types.ts web/src/lib/api.ts web/src/App.tsx web/src/pages/Standup.tsx
git commit -m "feat: add /standup web page with copy-as-markdown"
```

---

## Task 10: Autostart — systemd-user backend + `cockpit install --autostart`

**Files:**
- Create: `internal/platform/autostart/autostart.go`
- Create: `internal/platform/autostart/linux.go`
- Create: `internal/platform/autostart/noop.go`
- Create: `internal/platform/autostart/linux_test.go`
- Create: `internal/cli/install.go`

- [ ] **Step 10.1: Create `internal/platform/autostart/autostart.go`**

```go
// Package autostart manages OS-specific user-level autostart for spk-cockpit.
// Linux: systemd-user unit at ~/.config/systemd/user/spk-cockpit.service.
package autostart

// Backend installs/removes a user-level autostart entry.
type Backend interface {
	// Install writes the autostart entry pointing at exePath and enables it.
	Install(exePath string) error
	// Uninstall disables and removes the autostart entry. Idempotent.
	Uninstall() error
	// Status reports whether autostart is currently installed and enabled.
	Status() (Status, error)
}

// Status is the current autostart state.
type Status struct {
	Installed bool
	Enabled   bool
	Detail    string // free-form, e.g. "active" or "disabled"
}
```

- [ ] **Step 10.2: Create `internal/platform/autostart/linux.go`**

```go
//go:build linux

package autostart

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Linux implements Backend via systemd-user.
type Linux struct {
	unitDir  string // ~/.config/systemd/user
	unitFile string // spk-cockpit.service
}

// NewLinux constructs a Linux backend rooted at the user's $XDG_CONFIG_HOME or ~/.config.
func NewLinux() (*Linux, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	cfg := os.Getenv("XDG_CONFIG_HOME")
	if cfg == "" {
		cfg = filepath.Join(home, ".config")
	}
	return &Linux{
		unitDir:  filepath.Join(cfg, "systemd", "user"),
		unitFile: "spk-cockpit.service",
	}, nil
}

// Path returns the full path to the unit file (used for tests/inspection).
func (l *Linux) Path() string { return filepath.Join(l.unitDir, l.unitFile) }

const unitTemplate = `[Unit]
Description=spk-cockpit personal productivity tray
After=graphical-session.target

[Service]
ExecStart=%s start --foreground
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
`

// Install writes the unit file and runs systemctl --user daemon-reload + enable --now.
func (l *Linux) Install(exePath string) error {
	if err := os.MkdirAll(l.unitDir, 0o755); err != nil {
		return fmt.Errorf("mkdir unit dir: %w", err)
	}
	content := fmt.Sprintf(unitTemplate, exePath)
	if err := os.WriteFile(l.Path(), []byte(content), 0o644); err != nil { //nolint:gosec // unit file is intentionally world-readable
		return fmt.Errorf("write unit: %w", err)
	}
	if err := runSystemctl("daemon-reload"); err != nil {
		return err
	}
	if err := runSystemctl("enable", "--now", l.unitFile); err != nil {
		return err
	}
	return nil
}

// Uninstall disables, stops, and removes the unit file. Idempotent.
func (l *Linux) Uninstall() error {
	_ = runSystemctl("disable", "--now", l.unitFile) // ignore errors (may already be gone)
	if err := os.Remove(l.Path()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove unit: %w", err)
	}
	_ = runSystemctl("daemon-reload")
	return nil
}

// Status checks if the unit file exists and queries its enabled state.
func (l *Linux) Status() (Status, error) {
	if _, err := os.Stat(l.Path()); os.IsNotExist(err) {
		return Status{Installed: false}, nil
	} else if err != nil {
		return Status{}, err
	}
	st := Status{Installed: true}
	out, err := exec.Command("systemctl", "--user", "is-enabled", l.unitFile).CombinedOutput()
	st.Detail = string(out)
	if err == nil {
		st.Enabled = true
	}
	return st, nil
}

func runSystemctl(args ...string) error {
	cmd := exec.Command("systemctl", append([]string{"--user"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl %v: %w (%s)", args, err, string(out))
	}
	return nil
}
```

- [ ] **Step 10.3: Create `internal/platform/autostart/noop.go`**

```go
//go:build !linux

package autostart

import "errors"

// Noop is a fallback Backend for non-Linux platforms.
type Noop struct{}

// NewNoop returns a Noop backend.
func NewNoop() *Noop { return &Noop{} }

// Install returns ErrUnsupported.
func (Noop) Install(_ string) error { return errors.New("autostart: only supported on Linux in v1") }

// Uninstall returns ErrUnsupported.
func (Noop) Uninstall() error { return errors.New("autostart: only supported on Linux in v1") }

// Status reports as not installed.
func (Noop) Status() (Status, error) { return Status{}, nil }
```

- [ ] **Step 10.4: Create `internal/platform/autostart/linux_test.go`**

```go
//go:build linux

package autostart

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// We test only the parts that don't require a real systemd-user session:
// unit-file path resolution and unit-file content composition.
func TestLinux_Path_RespectsXdgConfigHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	l, err := NewLinux()
	require.NoError(t, err)
	require.Equal(t, filepath.Join(tmp, "systemd", "user", "spk-cockpit.service"), l.Path())
}

func TestLinux_UnitTemplate_ContainsExePath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	l, err := NewLinux()
	require.NoError(t, err)

	// Write directly to skip systemctl invocation.
	require.NoError(t, os.MkdirAll(l.unitDir, 0o755))
	require.NoError(t, os.WriteFile(l.Path(), []byte("placeholder"), 0o644))

	// Recreate via Install would try to invoke systemctl; instead, test the format string.
	content := unitTemplate
	require.True(t, strings.Contains(content, "ExecStart=%s start --foreground"))
	require.True(t, strings.Contains(content, "WantedBy=default.target"))
}
```

- [ ] **Step 10.5: Create `internal/cli/install.go`**

```go
package cli

import (
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/spk/spk-cockpit/internal/platform/autostart"
)

var installFlags struct {
	autostart bool
	uninstall bool
}

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install autostart and other OS-level integrations",
	RunE: func(_ *cobra.Command, _ []string) error {
		if !installFlags.autostart && !installFlags.uninstall {
			return fmt.Errorf("specify --autostart to install or --uninstall to remove")
		}
		var be autostart.Backend
		if runtime.GOOS == "linux" {
			lb, err := autostart.NewLinux()
			if err != nil {
				return err
			}
			be = lb
		} else {
			be = autostart.NewNoop()
		}

		if installFlags.uninstall {
			if err := be.Uninstall(); err != nil {
				return err
			}
			fmt.Println("autostart removed.")
			return nil
		}

		exePath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("locate executable: %w", err)
		}
		if err := be.Install(exePath); err != nil {
			return err
		}
		fmt.Println("autostart installed and enabled.")
		fmt.Printf("Unit: %s\n", exePath)
		return nil
	},
}

func init() {
	installCmd.Flags().BoolVar(&installFlags.autostart, "autostart", false, "Install the systemd-user autostart unit")
	installCmd.Flags().BoolVar(&installFlags.uninstall, "uninstall", false, "Remove the autostart unit")
	rootCmd.AddCommand(installCmd)
}
```

- [ ] **Step 10.6: Run + commit**

```bash
go build ./...
go test ./internal/platform/autostart/...
golangci-lint run ./internal/platform/autostart/... ./internal/cli/...
git add internal/platform/autostart/ internal/cli/install.go
git commit -m "feat: add cockpit install --autostart with systemd-user backend"
```

`TestLinux_*` tests must PASS (linux-only — they will not run on macOS/Windows; that is intentional).

---

## Task 11: GitHub Actions release workflow

**Files:**
- Create: `.github/workflows/release.yml`

- [ ] **Step 11.1: Create `.github/workflows/release.yml`**

```yaml
name: Release

on:
  push:
    tags:
      - "v*.*.*"

permissions:
  contents: write

jobs:
  build:
    name: Build ${{ matrix.target }}
    runs-on: ubuntu-latest
    strategy:
      fail-fast: false
      matrix:
        include:
          - target: linux-amd64
            goos: linux
            goarch: amd64
          - target: linux-arm64
            goos: linux
            goarch: arm64
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: "1.26"
          cache: true

      - uses: pnpm/action-setup@v4
        with:
          version: 10

      - uses: actions/setup-node@v4
        with:
          node-version: "20"
          cache: pnpm
          cache-dependency-path: web/pnpm-lock.yaml

      - name: Install Linux build deps
        run: |
          sudo apt-get update
          sudo apt-get install -y gcc pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev libsecret-1-dev

      - name: Build web
        run: |
          cd web
          pnpm install --frozen-lockfile
          pnpm build

      - name: Build cockpit binary
        env:
          GOOS: ${{ matrix.goos }}
          GOARCH: ${{ matrix.goarch }}
          CGO_ENABLED: "1"
        run: |
          mkdir -p dist
          go build -tags "webkit2_41 production" -trimpath \
            -ldflags "-s -w -X main.version=${GITHUB_REF_NAME}" \
            -o dist/spk-cockpit-${{ matrix.target }} ./cmd/cockpit

      - name: Compute checksum
        run: |
          cd dist
          sha256sum spk-cockpit-${{ matrix.target }} > spk-cockpit-${{ matrix.target }}.sha256

      - uses: actions/upload-artifact@v4
        with:
          name: spk-cockpit-${{ matrix.target }}
          path: dist/spk-cockpit-${{ matrix.target }}*

  release:
    name: Publish Release
    needs: build
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/download-artifact@v4
        with:
          path: dist
          merge-multiple: true

      - name: Create GitHub Release
        uses: softprops/action-gh-release@v2
        with:
          files: dist/*
          generate_release_notes: true
          fail_on_unmatched_files: true
```

**Notes for the executor:**

- `arm64` cross-compile with `CGO_ENABLED=1` requires a cross toolchain. If this fails on first try, drop arm64 from the matrix and revisit (modernc.org/sqlite is pure-Go but Wails/webkit binds to native GTK). For Phase 4, having `linux-amd64` released is acceptable; arm64 is best-effort.
- The workflow does NOT run tests (CI in a separate `ci.yml` is out of scope for Phase 4 — opportunistic if time permits).

- [ ] **Step 11.2: Commit**

```bash
mkdir -p .github/workflows
git add .github/workflows/release.yml
git commit -m "feat: add GitHub Actions release workflow"
```

(No automated test for this; verification happens at the next `v*.*.*` tag push.)

---

## Task 12: README + CHANGELOG + tag

**Files:**
- Modify: `README.md`

- [ ] **Step 12.1: Update README**

Add a new "Phase 4" section between the current "Phase 3" and "Architecture" sections (or wherever the README documents capabilities). The new section should mention:

```markdown
### Phase 4 — Standup helper, integrations, autostart, releases

- `cockpit standup` — prints today's standup as markdown (yesterday / today / blockers).
- Web `/standup` route with "Copy as markdown" button.
- Read-only GitLab integration: configure `gitlab.url` + `gitlab.author_username` in KV and store `gitlab_token` as a secret.
- Read-only Citeck Project Tracker integration: configure `tracker.url` + `tracker.username` and store `tracker_token` as a secret.
- `cockpit install --autostart` installs `~/.config/systemd/user/spk-cockpit.service` and enables it. Use `--uninstall` to remove.
- GitHub Actions release workflow on `v*.*.*` tags builds and publishes `linux/amd64` (and best-effort `linux/arm64`) binaries.

### Configuring GitLab and Tracker

```bash
# GitLab: store the personal access token in keyring-encrypted secret store.
cockpit secret set gitlab_token <PAT>
# Set base URL and author username via KV.
curl --unix-socket ~/.local/state/spk-cockpit/cockpit.sock \
     -X PUT -H 'Content-Type: application/json' \
     -d '{"value": "https://gitlab.example.com"}' http://unix/api/kv/gitlab.url
curl --unix-socket ~/.local/state/spk-cockpit/cockpit.sock \
     -X PUT -H 'Content-Type: application/json' \
     -d '{"value": "alice"}' http://unix/api/kv/gitlab.author_username
# Same pattern for tracker.url, tracker.username, tracker_token.
```
```

- [ ] **Step 12.2: Verify everything is green**

```bash
make test
cd web && pnpm test --run && cd ..
golangci-lint run
make build
```

Expected: all green; `build/bin/spk-cockpit` is produced.

- [ ] **Step 12.3: Tag and commit**

```bash
git add README.md
git commit -m "docs: update README for phase 4 completion"
git tag v0.4.0-phase4
```

(Do not push; user will do that.)

---

## Self-review checklist (run after writing all tasks)

1. **Spec coverage:**
   - §1 Standup helper → Task 4 (service) + Task 5 (markdown) + Task 6 (handler) + Task 8 (CLI) + Task 9 (web).
   - §3 `internal/sync/gitlab/` → Task 2.
   - §3 `internal/sync/tracker/` → Task 3.
   - §3 `internal/standup/` → Task 4–5.
   - §6 `/standup` route → Task 9.
   - §9 autostart unit → Task 10.
   - §8 release workflow → Task 11.

2. **No placeholders:** every task has full code blocks. No "TODO" / "implement later".

3. **Type consistency:**
   - `gitlab.Source.CommitsBy(author, since, until)` — used identically in standup service.
   - `tracker.Source.AssignedActive(username, since, until)` — used identically in standup service.
   - `api.StandupReport` shape mirrored between Go and TS (`web/src/lib/types.ts`).
   - `StandupItemSource` constants match between Go (`api.StandupSourceTodo` etc.) and TS literal types (`"todo" | "gitlab" | "tracker"`) — markdown formatter on both sides emits the same `[todo]` / `[git]` / `[pt]` tags.

4. **Conventions kept:** doc comments on all exported entities; `//nolint` comments where revive/gosec flag known-safe patterns; no `Co-Authored-By`; tags = ULID-style only where IDs are needed (Phase 4 has no new mutating IDs).

5. **DRY/YAGNI:**
   - No FTS5 (deferred to KB iteration).
   - No standup persistence (each call recomputes).
   - No webhook receivers (pull-only).
   - One markdown formatter in Go (used by HTTP+CLI), one in TS (used by web Copy button) — the duplication is intentional because the web button copies without a server round-trip.
