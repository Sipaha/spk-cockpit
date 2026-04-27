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

// ErrDaemonNotRunning indicates the daemon is unreachable. (Reserved for future use.)
var ErrDaemonNotRunning = errors.New("daemon not running")
