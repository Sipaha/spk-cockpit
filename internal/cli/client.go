package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/spk/spk-cockpit/internal/api"
)

// Client is a thin HTTP client that dials the cockpit daemon over its UDS.
type Client struct {
	httpClient *http.Client
}

// NewClient creates a Client connecting to socketPath.
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

// Health probes /api/health.
func (c *Client) Health(ctx context.Context) error {
	resp, err := c.do(ctx, "GET", "/api/health", nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	return nil
}

// ListTodos returns the daemon's todos.
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

// CreateTodo posts a new todo.
func (c *Client) CreateTodo(ctx context.Context, req api.CreateTodoRequest) (api.Todo, error) {
	var out api.Todo
	if err := c.postJSON(ctx, "/api/todos", req, &out); err != nil {
		return api.Todo{}, err
	}
	return out, nil
}

// UpdateTodo patches a todo.
func (c *Client) UpdateTodo(ctx context.Context, id string, req api.UpdateTodoRequest) (api.Todo, error) {
	var out api.Todo
	if err := c.patchJSON(ctx, "/api/todos/"+id, req, &out); err != nil {
		return api.Todo{}, err
	}
	return out, nil
}

// DeleteTodo deletes a todo.
func (c *Client) DeleteTodo(ctx context.Context, id string) error {
	resp, err := c.do(ctx, "DELETE", "/api/todos/"+id, nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
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
		defer func() { _ = resp.Body.Close() }()
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
	defer func() { _ = resp.Body.Close() }()
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
	defer func() { _ = resp.Body.Close() }()
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
	defer func() { _ = resp.Body.Close() }()
	return json.NewDecoder(resp.Body).Decode(out)
}

// StartTimer starts a timer on todoID.
func (c *Client) StartTimer(ctx context.Context, todoID string) (api.TimerSession, error) {
	var out api.TimerSession
	if err := c.postJSON(ctx, "/api/timer/start", api.StartTimerRequest{TodoID: todoID}, &out); err != nil {
		return api.TimerSession{}, err
	}
	return out, nil
}

// StopTimer stops the active timer.
func (c *Client) StopTimer(ctx context.Context) (api.TimerSession, error) {
	resp, err := c.do(ctx, "POST", "/api/timer/stop", nil)
	if err != nil {
		return api.TimerSession{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	var out api.TimerSession
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return api.TimerSession{}, err
	}
	return out, nil
}

// ActiveTimer returns the active session, or nil if none.
func (c *Client) ActiveTimer(ctx context.Context) (*api.TimerSession, error) {
	resp, err := c.do(ctx, "GET", "/api/timer/active", nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var out *api.TimerSession
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListMeetings lists upcoming meetings in [fromUnix, toUnix].
func (c *Client) ListMeetings(ctx context.Context, fromUnix, toUnix int64, includeCancelled bool) ([]api.Meeting, error) {
	q := fmt.Sprintf("/api/meetings?from=%d&to=%d", fromUnix, toUnix)
	if includeCancelled {
		q += "&includeCancelled=1"
	}
	var out []api.Meeting
	if err := c.getJSON(ctx, q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// NextMeeting returns the earliest upcoming meeting or nil.
func (c *Client) NextMeeting(ctx context.Context) (*api.Meeting, error) {
	resp, err := c.do(ctx, "GET", "/api/meetings/next", nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var out *api.Meeting
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// SetSecret encrypts and stores a value under name.
func (c *Client) SetSecret(ctx context.Context, name, value string) error {
	b, err := json.Marshal(api.SetSecretRequest{Value: value})
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	resp, err := c.do(ctx, "PUT", "/api/secrets/"+name, bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	return nil
}

// ListSecretNames returns all known secret names (no values).
func (c *Client) ListSecretNames(ctx context.Context) ([]api.Secret, error) {
	var out []api.Secret
	if err := c.getJSON(ctx, "/api/secrets", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// TriggerSync forces a sync for the named source (e.g. "caldav").
func (c *Client) TriggerSync(ctx context.Context, source string) error {
	resp, err := c.do(ctx, "POST", "/api/sync/"+source, nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	return nil
}

