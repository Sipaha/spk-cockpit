package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/spk/spk-cockpit/internal/clock"
	"github.com/spk/spk-cockpit/internal/eventbus"
	cockpitlog "github.com/spk/spk-cockpit/internal/log"
	"github.com/spk/spk-cockpit/internal/paths"
	"github.com/spk/spk-cockpit/internal/server"
	"github.com/spk/spk-cockpit/internal/store"
	"github.com/spk/spk-cockpit/internal/timer"
	"github.com/spk/spk-cockpit/internal/todo"
	"github.com/spk/spk-cockpit/internal/tray"
	"github.com/spk/spk-cockpit/internal/window"
	webembed "github.com/spk/spk-cockpit/web/embed"
)

var startFlags struct {
	foreground bool
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the spk-cockpit daemon",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runStart(cmd.Context())
	},
}

func init() {
	startCmd.Flags().BoolVar(&startFlags.foreground, "foreground", false, "Run in foreground (do not fork)")
	rootCmd.AddCommand(startCmd)
}

func runStart(ctx context.Context) error {
	logger := cockpitlog.New(os.Stderr, cockpitlog.ParseLevel(os.Getenv("SPK_COCKPIT_LOG_LEVEL")))

	p, err := paths.New()
	if err != nil {
		return fmt.Errorf("paths: %w", err)
	}

	pidFile := filepath.Join(p.StateDir, "cockpit.pid")
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0o600); err != nil {
		return fmt.Errorf("write pid file: %w", err)
	}
	defer os.Remove(pidFile) //nolint:errcheck

	logger.Info("starting spk-cockpit", "data", p.DataDir, "socket", p.SocketFile)

	st, err := store.Open("file:" + p.DBFile)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer st.Close() //nolint:errcheck
	if err := store.Migrate(st.DB); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	bus := eventbus.New(64)
	defer bus.Close()

	todoRepo := store.NewTodoRepo(st.DB)
	tagRepo := store.NewTagRepo(st.DB)
	eventRepo := store.NewEventRepo(st.DB)
	todoSvc := todo.NewService(todoRepo, tagRepo, eventRepo, clock.Real(), bus)

	timerRepo := store.NewTimerRepo(st.DB)
	timerSvc := timer.NewService(timerRepo, clock.Real(), bus)

	srv, err := server.New(server.Config{SocketPath: p.SocketFile, Logger: logger})
	if err != nil {
		return fmt.Errorf("server: %w", err)
	}
	srv.Deps().Todos = todoSvc
	srv.Deps().Tags = tagRepo
	srv.Deps().Bus = bus
	srv.Deps().Timer = timerSvc

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("server listening", "socket", p.SocketFile)
		serveErr <- srv.Serve()
	}()

	go func() { //nolint:gosec // context.Background is intentional: Stop needs its own deadline, not the cancelled request ctx
		<-ctx.Done()
		logger.Info("shutting down")
		_ = srv.Stop(context.Background())
	}()

	// Tray runs in a goroutine and forwards "Open window" / "Quit" to handlers.
	var winApp *window.App
	go func() {
		t := tray.New(
			func() {
				if winApp != nil {
					winApp.Show()
				}
			},
			func() {
				cancel()
			},
		)
		t.Run(nil, nil)
	}()

	// Wails owns the main thread.
	winErr := window.Run(webembed.DistFS, p.SocketFile, func(a *window.App) {
		winApp = a
	})
	logger.Info("window closed", "err", winErr)

	cancel()
	_ = srv.Stop(context.Background())
	if err := <-serveErr; err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	return winErr
}
