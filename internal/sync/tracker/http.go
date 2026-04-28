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

// HTTPSource calls a tracker's records query API to fetch user-assigned items.
//
//nolint:revive // HTTPSource is not a stutter; "HTTP" is a qualifier, not the package name.
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
	SourceID    string         `json:"sourceId"`
	Query       map[string]any `json:"query"`
	GroupBy     []string       `json:"groupBy,omitempty"`
	Consistency string         `json:"consistency,omitempty"`
}

type pageBody struct {
	MaxItems int `json:"maxItems"`
	Skip     int `json:"skip"`
}

type queryResponse struct {
	Records []struct {
		ID         string `json:"id"`
		Attributes struct {
			DispName   string `json:"_disp"`
			Status     string `json:"_status"`
			ModifiedAt string `json:"_modified"`
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
		at, err := time.Parse(time.RFC3339, r.Attributes.ModifiedAt)
		if err != nil {
			// skip records with unparseable modified timestamps; rare and not worth surfacing
			continue
		}
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
