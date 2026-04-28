package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/clock"
	"github.com/spk/spk-cockpit/internal/eventbus"
	cockpitlog "github.com/spk/spk-cockpit/internal/log"
	"github.com/spk/spk-cockpit/internal/meeting"
	"github.com/spk/spk-cockpit/internal/note"
	"github.com/spk/spk-cockpit/internal/notify"
	"github.com/spk/spk-cockpit/internal/paths"
	"github.com/spk/spk-cockpit/internal/popup"
	"github.com/spk/spk-cockpit/internal/secret"
	"github.com/spk/spk-cockpit/internal/server"
	"github.com/spk/spk-cockpit/internal/standup"
	"github.com/spk/spk-cockpit/internal/store"
	"github.com/spk/spk-cockpit/internal/sync/caldav"
	"github.com/spk/spk-cockpit/internal/sync/gitlab"
	"github.com/spk/spk-cockpit/internal/sync/tracker"
	"github.com/spk/spk-cockpit/internal/timer"
	"github.com/spk/spk-cockpit/internal/todo"
	"github.com/spk/spk-cockpit/internal/tray"
	"github.com/spk/spk-cockpit/internal/window"
	webembed "github.com/spk/spk-cockpit/web/embed"
)

func init() {
	rootCmd.RunE = func(cmd *cobra.Command, _ []string) error {
		return runStart(cmd.Context())
	}
}

func runStart(ctx context.Context) error {
	logger := cockpitlog.New(os.Stderr, cockpitlog.ParseLevel(os.Getenv("SPK_COCKPIT_LOG_LEVEL")))
	slog.SetDefault(logger) // so slog.Debug(...) elsewhere honors SPK_COCKPIT_LOG_LEVEL

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

	kv := store.NewKvRepo(st.DB)
	gitlabSrc := buildGitLabSource(ctx, kv, secretSvc, logger)
	trackerSrc := buildTrackerSource(ctx, kv, secretSvc, logger)
	glAuthor, _, _ := kv.Get(ctx, "gitlab.author_username")
	trackerUser, _, _ := kv.Get(ctx, "tracker.username")

	standupSvc := standup.NewService(standup.Config{
		Todos:        todoSvc,
		Events:       eventRepo,
		GitLab:       gitlabSrc,
		GitLabAuthor: glAuthor,
		Tracker:      trackerSrc,
		TrackerUser:  trackerUser,
		Clock:        clock.Real(),
	})
	srv.Deps().Standup = standupSvc

	// CalDAV syncer is created lazily so that saving credentials in the running
	// Settings UI takes effect without a daemon restart. The first call to
	// /api/sync/caldav (or a successful poll) constructs the real syncer and
	// starts its background Run goroutine. Subsequent calls reuse the same instance.
	var (
		caldavSyncer *caldav.Syncer
		caldavMu     sync.Mutex
	)
	mintCaldav := func() *caldav.Syncer {
		caldavMu.Lock()
		defer caldavMu.Unlock()
		if caldavSyncer != nil {
			return caldavSyncer
		}
		cfg := loadCaldavConfig(secretSvc, st.DB)
		if cfg == nil {
			return nil
		}
		client, err := caldav.NewClient(*cfg)
		if err != nil {
			logger.Warn("caldav client init failed; sync disabled", "err", err)
			return nil
		}
		s := caldav.NewSyncer(caldav.SyncerConfig{
			Client:   client,
			Meetings: meetingSvc,
			State:    syncStateRepo,
			Clock:    clock.Real(),
			Logger:   logger,
			Bus:      bus,
		})
		go s.Run(ctx)
		caldavSyncer = s
		logger.Info("caldav syncer initialized")
		return s
	}
	mintCaldav() // try at startup; harmless if config is missing
	srv.Deps().Sync = &lazySync{mint: mintCaldav}

	var notifier notify.Notifier
	dbusN, err := notify.NewDBus()
	if err != nil {
		logger.Warn("dbus init failed; using noop notifier", "err", err)
		notifier = notify.NewNoop(logger)
	} else {
		notifier = dbusN
	}
	defer func() { _ = notifier.Close() }()

	defaultNotifyMin := 5
	if v, ok, _ := store.NewKvRepo(st.DB).Get(ctx, "meeting.default_notify_min"); ok {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			defaultNotifyMin = n
		}
	}
	defaultPopupMin := 1
	if v, ok, _ := store.NewKvRepo(st.DB).Get(ctx, "meeting.default_popup_min"); ok {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			defaultPopupMin = n
		}
	}

	exePath, err := os.Executable()
	if err != nil {
		logger.Warn("os.Executable failed; meeting popup disabled", "err", err)
		exePath = ""
	}
	popupBackend := popup.NewSubprocess(exePath, p.SocketFile, logger)
	if !popupBackend.Available() {
		logger.Warn("meeting popup disabled (no executable path)")
	}

	scheduler := notify.NewScheduler(notify.SchedulerConfig{
		Meetings: meetingSvc,
		Notifier: notifier,
		Popup: func(m api.Meeting) {
			popupBackend.Show(m)
		},
		Clock:            clock.Real(),
		Logger:           logger,
		DefaultNotifyMin: defaultNotifyMin,
		DefaultPopupMin:  defaultPopupMin,
	})
	go scheduler.Run(ctx)

	// winApp is bound by window.Run below; tray click handlers reference it via
	// closure so they pick up the live handle once the window is up.
	var winApp *window.App
	// trayBackend is declared up-front so the shutdown goroutine (defined before
	// the tray action wiring) can call into it via closure.
	var trayBackend tray.Backend

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("server listening", "socket", p.SocketFile)
		serveErr <- srv.Serve()
	}()

	go func() { //nolint:gosec // context.Background is intentional: Stop needs its own deadline, not the cancelled request ctx
		<-ctx.Done()
		logger.Info("shutting down")
		// Kick off Wails and tray teardown in parallel with the HTTP shutdown so
		// the user-perceived Quit latency is just whichever is slowest, not their sum.
		if winApp != nil {
			winApp.Quit()
		}
		if trayBackend != nil {
			trayBackend.Quit()
		}
		_ = srv.Stop(context.Background())
	}()

	// Tray runs in a goroutine. Click handlers call back into the daemon's services.
	// winApp is shared with the popup callback above (declared near the scheduler).
	trayActions := tray.Actions{
		OpenWindow: func() {
			if winApp != nil {
				winApp.Show()
			}
		},
		OpenStandup: func() {
			if winApp != nil {
				winApp.ShowAt("/standup")
			}
		},
		StopTimer: func() {
			ctx := context.Background()
			active, err := timerSvc.Active(ctx)
			if err != nil {
				logger.Warn("tray: list active failed", "err", err)
				return
			}
			for _, a := range active {
				if _, _, err := timerSvc.Stop(ctx, a.TodoID); err != nil {
					logger.Warn("tray: stop timer failed", "todo", a.TodoID, "err", err)
				}
			}
		},
		QuickAddTodo: func() {
			if exePath == "" {
				logger.Warn("tray: quick-add disabled (no executable path)")
				return
			}
			cmd := exec.Command(exePath, "quick-add", "--socket="+p.SocketFile) //nolint:gosec
			if err := cmd.Start(); err != nil {
				logger.Warn("tray: quick-add subprocess start failed", "err", err)
				return
			}
			// Reap the child so it doesn't linger as zombie when the window closes.
			go func() { _ = cmd.Wait() }()
		},
		OpenMeeting: func(id string) {
			if winApp != nil {
				winApp.ShowAt("/calendar?focus=" + id)
			}
		},
		Quit: func() {
			cancel()
		},
	}
	trayBackend = tray.New(trayActions)
	go func() {
		trayBackend.Run(nil, nil)
	}()
	mtgFetch := func() *api.Meeting {
		m, err := meetingSvc.Next(context.Background())
		if err != nil {
			return nil
		}
		return m
	}
	go tray.NewSubscriber(bus, trayBackend, todoSvc, mtgFetch).Run(ctx)

	geomKv := store.NewKvRepo(st.DB)
	loadGeometry := func() *window.Geometry {
		v, ok, err := geomKv.Get(context.Background(), "window.geometry")
		if err != nil || !ok || v == "" {
			return nil
		}
		var g window.Geometry
		if err := json.Unmarshal([]byte(v), &g); err != nil {
			return nil
		}
		return &g
	}
	saveGeometry := func(g window.Geometry) {
		b, err := json.Marshal(g)
		if err != nil {
			return
		}
		if err := geomKv.Set(context.Background(), "window.geometry", string(b)); err != nil {
			logger.Warn("save window geometry failed", "err", err)
		}
	}

	// Wails owns the main thread.
	winErr := window.Run(webembed.DistFS, p.SocketFile, func(a *window.App) {
		winApp = a
	}, loadGeometry, saveGeometry)
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
	password, err := secrets.Get(ctx, "caldav_password")
	if err != nil || password == "" {
		return nil
	}
	return &caldav.Config{BaseURL: url, Username: username, Password: password}
}

// buildGitLabSource constructs a GitLab sync source from KV config and secrets.
// Returns nil if any required value is missing or init fails.
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

// lazySync defers caldav syncer construction to first use, then memoizes the
// instance. Saving credentials in Settings can therefore activate sync without
// a daemon restart: the next /api/sync/caldav call mints the real syncer.
type lazySync struct {
	mint func() *caldav.Syncer
}

// TriggerNow forwards to the underlying syncer, minting it on demand.
func (l *lazySync) TriggerNow(source string) error {
	s := l.mint()
	if s == nil {
		return errSyncNotConfigured
	}
	return s.TriggerNow(source)
}

// Status returns the underlying syncer's status, or an empty list if not yet minted.
func (l *lazySync) Status() []api.SyncStateEntry {
	s := l.mint()
	if s == nil {
		return nil
	}
	return s.Status()
}

var errSyncNotConfigured = fmt.Errorf("sync not configured: missing caldav.url, caldav.username or caldav_password")

// buildTrackerSource constructs a tracker sync source from KV config and secrets.
// Returns nil if any required value is missing or init fails.
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
