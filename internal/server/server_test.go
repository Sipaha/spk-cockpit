package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/clock"
	"github.com/spk/spk-cockpit/internal/eventbus"
	"github.com/spk/spk-cockpit/internal/server"
	"github.com/spk/spk-cockpit/internal/timer"
	timerfake "github.com/spk/spk-cockpit/internal/timer/fakerepo"
	"github.com/spk/spk-cockpit/internal/todo"
	"github.com/spk/spk-cockpit/internal/todo/fakerepo"
)

func TestServer_HealthEndpoint(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "test.sock")
	srv, err := server.New(server.Config{SocketPath: sock})
	require.NoError(t, err)
	go func() { _ = srv.Serve() }()
	defer func() { _ = srv.Stop(context.Background()) }()
	waitForSocket(t, sock)

	c := &http.Client{Transport: &http.Transport{
		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", sock)
		},
	}}
	resp, err := c.Get("http://unix/api/health")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
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

func newTestServer(t *testing.T) (string, func()) {
	sock := filepath.Join(t.TempDir(), "test.sock")
	srv, err := server.New(server.Config{SocketPath: sock})
	require.NoError(t, err)
	tr, gr, er := fakerepo.NewTodo(), fakerepo.NewTag(), fakerepo.NewEvent()
	bus := eventbus.New(8)
	srv.Deps().Todos = todo.NewService(tr, gr, er, clock.NewFake(time.Unix(1700000000, 0)), bus)
	srv.Deps().Tags = gr
	srv.Deps().Bus = bus
	timerRepo := timerfake.NewTimer()
	srv.Deps().Timer = timer.NewService(timerRepo, clock.NewFake(time.Unix(1700000000, 0)), bus)
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
	_ = resp.Body.Close()

	resp, err = c.Get("http://unix/api/todos")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
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

func TestServer_SSEReceivesPublishedEvents(t *testing.T) {
	sock, stop := newTestServer(t)
	defer stop()

	c := udsClient(sock)
	req, _ := http.NewRequest("GET", "http://unix/api/events", nil)
	resp, err := c.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, 200, resp.StatusCode)

	body, _ := json.Marshal(api.CreateTodoRequest{Title: "evt-test", Priority: api.PriorityNormal})
	postResp, err := c.Post("http://unix/api/todos", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	_ = postResp.Body.Close()

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
}

func TestServer_TimerStartActiveStop(t *testing.T) {
	sock, stop := newTestServer(t)
	defer stop()
	c := udsClient(sock)

	body, _ := json.Marshal(api.CreateTodoRequest{Title: "T", Priority: api.PriorityNormal})
	resp, err := c.Post("http://unix/api/todos", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	var td api.Todo
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&td))

	startBody, _ := json.Marshal(api.StartTimerRequest{TodoID: td.ID})
	resp2, err := c.Post("http://unix/api/timer/start", "application/json", bytes.NewReader(startBody))
	require.NoError(t, err)
	defer func() { _ = resp2.Body.Close() }()
	require.Equal(t, 200, resp2.StatusCode)

	resp3, err := c.Get("http://unix/api/timer/active")
	require.NoError(t, err)
	defer func() { _ = resp3.Body.Close() }()
	var active *api.TimerSession
	require.NoError(t, json.NewDecoder(resp3.Body).Decode(&active))
	require.NotNil(t, active)
	require.Equal(t, td.ID, active.TodoID)

	resp4, err := c.Post("http://unix/api/timer/stop", "application/json", nil)
	require.NoError(t, err)
	defer func() { _ = resp4.Body.Close() }()
	require.Equal(t, 200, resp4.StatusCode)

	resp5, err := c.Get("http://unix/api/timer/active")
	require.NoError(t, err)
	defer func() { _ = resp5.Body.Close() }()
	var afterStop *api.TimerSession
	require.NoError(t, json.NewDecoder(resp5.Body).Decode(&afterStop))
	require.Nil(t, afterStop)
}
