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
//
//nolint:revive // HTTPSource is not a stutter; "HTTP" is a qualifier, not the package name.
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
		if e.ProjectFullPath == "" {
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
	out := events[:0] // reuse backing array; events is not referenced after this point
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
