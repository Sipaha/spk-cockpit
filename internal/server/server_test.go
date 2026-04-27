package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/clock"
	"github.com/spk/spk-cockpit/internal/eventbus"
	"github.com/spk/spk-cockpit/internal/server"
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
