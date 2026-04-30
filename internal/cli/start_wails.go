//go:build wails

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/appfiles"
	"github.com/spk/spk-cockpit/internal/clock"
	"github.com/spk/spk-cockpit/internal/desktop"
	"github.com/spk/spk-cockpit/internal/eventbus"
	cockpitlog "github.com/spk/spk-cockpit/internal/log"
	"github.com/spk/spk-cockpit/internal/meeting"
	"github.com/spk/spk-cockpit/internal/note"
	"github.com/spk/spk-cockpit/internal/notify"
	"github.com/spk/spk-cockpit/internal/paths"
	"github.com/spk/spk-cockpit/internal/secret"
	"github.com/spk/spk-cockpit/internal/server"
	"github.com/spk/spk-cockpit/internal/store"
	"github.com/spk/spk-cockpit/internal/sync/caldav"
	"github.com/spk/spk-cockpit/internal/timer"
	"github.com/spk/spk-cockpit/internal/todo"
	"github.com/spk/spk-cockpit/internal/tray"
)

func runStart(ctx context.Context) error {
	logger := cockpitlog.New(os.Stderr, cockpitlog.ParseLevel(os.Getenv("SPK_COCKPIT_LOG_LEVEL")))
	slog.SetDefault(logger)

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

	bus := eventbus.New()
	// publisherWg tracks goroutines that may call bus.Publish (scheduler,
	// caldav syncer, HTTP handlers). bus.Close must run AFTER these unwind,
	// otherwise a late Publish races a closed channel.
	var publisherWg sync.WaitGroup

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
	srv.Deps().Tags = todo.NewTagService(tagRepo, clock.Real(), bus)
	srv.Deps().Bus = bus
	srv.Deps().Timer = timerSvc

	meetingRepo := store.NewMeetingRepo(st.DB)
	noteRepo := store.NewNoteRepo(st.DB)
	secretRepo := store.NewSecretRepo(st.DB)
	syncStateRepo := store.NewSyncStateRepo(st.DB)

	meetingSvc := meeting.NewService(meetingRepo, clock.Real(), bus)
	noteSvc := note.NewService(noteRepo, clock.Real())

	masterKey, err := secret.ResolveOrFallback(secret.NewKeyringResolver(), secret.NewEnvResolver(""))
	if err != nil {
		return fmt.Errorf("master key: %w", err)
	}
	secretSvc, err := secret.NewService(secretRepo, clock.Real(), masterKey)
	if err != nil {
		return fmt.Errorf("secret service: %w", err)
	}

	kv := store.NewKvRepo(st.DB)
	srv.Deps().Meetings = meetingSvc
	srv.Deps().Notes = noteSvc
	srv.Deps().Secrets = secretSvc
	srv.Deps().Kv = kv
	srv.Deps().Clock = clock.Real()

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// CalDAV syncer is created lazily so that saving credentials in the running
	// Settings UI takes effect without a daemon restart.
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
		cfg := loadCaldavConfig(secretSvc, kv)
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
		publisherWg.Add(1)
		go func() {
			defer publisherWg.Done()
			s.Run(ctx)
		}()
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
	if v, ok, _ := kv.Get(ctx, "meeting.default_notify_min"); ok {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			defaultNotifyMin = n
		}
	}
	defaultPopupMin := 1
	if v, ok, _ := kv.Get(ctx, "meeting.default_popup_min"); ok {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			defaultPopupMin = n
		}
	}

	// openPopup is the indirection to the Wails-owned in-process popup. The
	// scheduler captures popupCallback below; the callback dereferences this
	// atomic pointer at fire-time. Until Wails finishes startup, the load
	// returns nil and the popup is silently skipped.
	var openPopup atomic.Pointer[func(string)]
	popupCallback := func(m api.Meeting) {
		if fn := openPopup.Load(); fn != nil {
			(*fn)(m.ID)
		}
	}

	scheduler := notify.NewScheduler(notify.SchedulerConfig{
		Meetings:         meetingSvc,
		Notifier:         notifier,
		Popup:            popupCallback,
		Clock:            clock.Real(),
		Logger:           logger,
		DefaultNotifyMin: defaultNotifyMin,
		DefaultPopupMin:  defaultPopupMin,
	})
	publisherWg.Add(1)
	go func() {
		defer publisherWg.Done()
		scheduler.Run(ctx)
	}()

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("server listening", "socket", p.SocketFile)
		serveErr <- srv.Serve()
	}()

	// Drops the v2 winApp.Quit polling loop — desktop.Run owns the ctx→Quit
	// goroutine internally (window.go's select <-ctx.Done(): app.Quit()).
	// We just need to call srv.Stop here on cancellation.
	go func() { //nolint:gosec
		<-ctx.Done()
		logger.Info("shutting down")
		_ = srv.Stop(context.Background())
	}()

	// Geometry persistence — KV-backed.
	loadGeometry := func() *desktop.Geometry {
		v, ok, err := kv.Get(context.Background(), "window.geometry")
		if err != nil || !ok || v == "" {
			return nil
		}
		var g desktop.Geometry
		if err := json.Unmarshal([]byte(v), &g); err != nil {
			return nil
		}
		return &g
	}
	saveGeometry := func(g desktop.Geometry) {
		b, err := json.Marshal(g)
		if err != nil {
			return
		}
		if err := kv.Set(context.Background(), "window.geometry", string(b)); err != nil {
			logger.Warn("save window geometry failed", "err", err)
		}
	}

	mtgFetch := func() *api.Meeting {
		m, err := meetingSvc.Next(context.Background())
		if err != nil {
			return nil
		}
		return m
	}

	onReady := func(
		app *application.App,
		main desktop.WindowHandle,
		openQuickAdd func(),
		openMeetingPopup func(string),
	) {
		// Stash the popup callback so the scheduler's popupCallback closure
		// can fire it once the window is up. Using atomic.Pointer matches the
		// audit-fix pattern: popupCallback runs from the scheduler goroutine,
		// onReady runs from the Wails OnStartup goroutine — both must order
		// without an explicit mutex.
		openPopup.Store(&openMeetingPopup)

		trayActions := tray.Actions{
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
			QuickAddTodo: openQuickAdd,
			OpenMeeting: func(id string) {
				main.Show()
				main.Navigate("/calendar?focus=" + id)
			},
			Quit: cancel,
		}

		// appfiles.TrayIcon (icons/tray.png) is the small, system-tray-sized icon variant.
		// appfiles.AppIcon (icons/appicon.png) is the larger title-bar variant.
		c, err := tray.NewController(app, main, appfiles.TrayIcon, trayActions)
		if err != nil {
			logger.Warn("tray: controller init failed", "err", err)
			return
		}
		c.Run(ctx, bus, todoSvc, mtgFetch)
	}

	// Wails owns the main thread.
	winErr := desktop.Run(ctx, desktop.Options{
		FrontendFS:   pkgFrontendFS,
		SocketPath:   p.SocketFile,
		IconPNG:      appfiles.AppIcon,
		LoadGeometry: loadGeometry,
		SaveGeometry: saveGeometry,
		OnReady:      onReady,
	})
	logger.Info("window closed", "err", winErr)

	// Ordered shutdown (matches the invariant documented on Server.Stop):
	//   cancel ctx → drain HTTP → wait for publishers → close bus.
	cancel()
	_ = srv.Stop(context.Background())
	if err := <-serveErr; err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	publisherWg.Wait()
	bus.Close()
	return winErr
}

// loadCaldavConfig reads CalDAV credentials from the KV store and secret service.
// Returns nil if any required value is missing or an error occurs.
func loadCaldavConfig(secrets *secret.Service, kv todo.KvRepo) *caldav.Config {
	ctx := context.Background()
	url, _, err := kv.Get(ctx, "caldav.url")
	if err != nil || url == "" {
		return nil
	}
	username, _, err := kv.Get(ctx, "caldav.username")
	if err != nil || username == "" {
		return nil
	}
	password, err := secrets.Get(ctx, "caldav_password")
	if err != nil || password == "" {
		return nil
	}
	return &caldav.Config{BaseURL: url, Username: username, Password: password}
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
