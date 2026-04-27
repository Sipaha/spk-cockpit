package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/spk/spk-cockpit/internal/clock"
	"github.com/spk/spk-cockpit/internal/eventbus"
	cockpitlog "github.com/spk/spk-cockpit/internal/log"
	"github.com/spk/spk-cockpit/internal/meeting"
	"github.com/spk/spk-cockpit/internal/note"
	"github.com/spk/spk-cockpit/internal/paths"
	"github.com/spk/spk-cockpit/internal/secret"
	"github.com/spk/spk-cockpit/internal/server"
	"github.com/spk/spk-cockpit/internal/store"
	"github.com/spk/spk-cockpit/internal/sync/caldav"
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

	meetingRepo := store.NewMeetingRepo(st.DB)
	noteRepo := store.NewNoteRepo(st.DB)
	secretRepo := store.NewSecretRepo(st.DB)
	syncStateRepo := store.NewSyncStateRepo(st.DB)

	meetingSvc := meeting.NewService(meetingRepo, clock.Real(), bus)
	noteSvc := note.NewService(noteRepo, clock.Real(), bus)

	masterKey, err := secret.ResolveOrFallback(secret.NewKeyringResolver(), secret.NewEnvResolver(""))
	if err != nil {
		return fmt.Errorf("master key: %w", err)
	}
	secretSvc, err := secret.NewService(secretRepo, clock.Real(), masterKey)
	if err != nil {
		return fmt.Errorf("secret service: %w", err)
	}

	srv.Deps().Meetings = meetingSvc
	srv.Deps().Notes = noteSvc
	srv.Deps().Secrets = secretSvc
	srv.Deps().Kv = store.NewKvRepo(st.DB)

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	caldavCfg := loadCaldavConfig(secretSvc, st.DB)
	var caldavSyncer *caldav.Syncer
	if caldavCfg != nil {
		client, err := caldav.NewClient(*caldavCfg)
		if err != nil {
			logger.Warn("caldav client init failed; sync disabled", "err", err)
		} else {
			caldavSyncer = caldav.NewSyncer(caldav.SyncerConfig{
				Client:   client,
				Meetings: meetingSvc,
				State:    syncStateRepo,
				Clock:    clock.Real(),
				Logger:   logger,
				Bus:      bus,
			})
		}
	}
	if caldavSyncer != nil {
		go caldavSyncer.Run(ctx)
		srv.Deps().Sync = caldavSyncer
	}

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
	trayBackend := tray.New(
		func() {
			if winApp != nil {
				winApp.Show()
			}
		},
		func() {
			cancel()
		},
	)
	go func() {
		trayBackend.Run(nil, nil)
	}()
	go tray.NewSubscriber(bus, trayBackend).Run(ctx)

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

// loadCaldavConfig reads CalDAV credentials from the KV store and secret service.
// Returns nil if any required value is missing or an error occurs.
func loadCaldavConfig(secrets *secret.Service, db *sql.DB) *caldav.Config {
	ctx := context.Background()
	url, _, err := store.NewKvRepo(db).Get(ctx, "caldav.url")
	if err != nil || url == "" {
		return nil
	}
	username, _, err := store.NewKvRepo(db).Get(ctx, "caldav.username")
	if err != nil || username == "" {
		return nil
	}
	password, err := secrets.Get(ctx, "yandex_caldav")
	if err != nil {
		return nil
	}
	return &caldav.Config{BaseURL: url, Username: username, Password: password}
}
