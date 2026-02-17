package daemon

import (
	"context"

	"github.com/matheus3301/wpp/internal/api"
	"github.com/matheus3301/wpp/internal/bus"
	"github.com/matheus3301/wpp/internal/lock"
	"github.com/matheus3301/wpp/internal/logging"
	"github.com/matheus3301/wpp/internal/outbox"
	"github.com/matheus3301/wpp/internal/session"
	"github.com/matheus3301/wpp/internal/status"
	"github.com/matheus3301/wpp/internal/store"
	intsync "github.com/matheus3301/wpp/internal/sync"
	"github.com/matheus3301/wpp/internal/wa"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// Params holds the resolved session configuration passed to the fx module.
type Params struct {
	SessionName string
	SocketPath  string // optional override for testing; empty = use default
}

// Module returns the fx module for the daemon, composing all providers and lifecycle hooks.
func Module(p Params) fx.Option {
	return fx.Module("daemon",
		fx.Supply(p),
		fx.Provide(
			provideLogger,
			provideBus,
			provideStateMachine,
			provideLock,
			provideStore,
			provideAdapter,
			provideSyncEngine,
			provideSender,
			provideSessionService,
			provideSyncService,
			provideChatService,
			provideMessageService,
			NewServer,
		),
		fx.Invoke(registerLifecycle),
	)
}

func provideLogger(p Params) (*zap.Logger, error) {
	return logging.New(session.LogPath(p.SessionName), p.SessionName)
}

func provideBus() *bus.Bus {
	return bus.New()
}

func provideStateMachine(b *bus.Bus) *status.Machine {
	return status.NewMachine(b)
}

func provideLock(p Params, logger *zap.Logger) (*lock.Lock, error) {
	if err := session.EnsureDir(p.SessionName); err != nil {
		return nil, err
	}
	logger.Info("acquiring session lock", zap.String("session", p.SessionName))
	l, err := lock.Acquire(session.Dir(p.SessionName))
	if err != nil {
		return nil, err
	}
	logger.Info("session lock acquired")
	return l, nil
}

func provideStore(p Params, logger *zap.Logger) (*store.DB, error) {
	dbPath := session.AppDBPath(p.SessionName)
	db, err := store.Open(dbPath)
	if err != nil {
		return nil, err
	}
	result, err := db.Migrate()
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	if result.Changed {
		logger.Info("migrations applied", zap.Uint("version", result.Version))
	} else {
		logger.Info("migrations up to date", zap.Uint("version", result.Version))
	}
	logger.Info("store initialized", zap.String("path", dbPath))
	return db, nil
}

func provideAdapter(p Params, b *bus.Bus, logger *zap.Logger) (*wa.Adapter, error) {
	return wa.NewAdapter(context.Background(), p.SessionName, b, logger)
}

func provideSyncEngine(db *store.DB, b *bus.Bus, logger *zap.Logger) *intsync.Engine {
	return intsync.NewEngine(db, b, logger)
}

func provideSender(db *store.DB, adapter *wa.Adapter, b *bus.Bus, logger *zap.Logger) *outbox.Sender {
	return outbox.NewSender(db, adapter, b, logger)
}

func provideSessionService(p Params, m *status.Machine, adapter *wa.Adapter, b *bus.Bus) *api.SessionService {
	return api.NewSessionService(p.SessionName, m, adapter, b)
}

func provideSyncService(adapter *wa.Adapter, b *bus.Bus) *api.SyncService {
	return api.NewSyncService(adapter, b)
}

func provideChatService(db *store.DB, b *bus.Bus) *api.ChatService {
	return api.NewChatService(db, b)
}

func provideMessageService(db *store.DB, b *bus.Bus) *api.MessageService {
	return api.NewMessageService(db, b)
}

func registerLifecycle(lc fx.Lifecycle, srv *Server, lk *lock.Lock, adapter *wa.Adapter, engine *intsync.Engine, sender *outbox.Sender, machine *status.Machine, b *bus.Bus, logger *zap.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			// Start sync engine (subscribes to wa.* bus events).
			engine.Start(context.Background())

			// Register event handler for whatsmeow events.
			handler := wa.NewEventHandler(b, machine, logger)
			adapter.RegisterEventHandler(handler.Handle)

			// Start gRPC server in background.
			go func() {
				if err := srv.Start(); err != nil {
					logger.Error("gRPC server error", zap.Error(err))
				}
			}()

			// Start outbox sender.
			sender.Start(context.Background())

			// Transition state based on auth status.
			if adapter.IsLoggedIn() {
				_ = machine.Transition(status.Connecting)
				go func() {
					if err := adapter.Connect(); err != nil {
						logger.Error("auto-connect failed", zap.Error(err))
						_ = machine.Transition(status.Error)
					}
				}()
			} else {
				logger.Info("no credentials found, auth required")
				_ = machine.Transition(status.AuthRequired)
			}

			return nil
		},
		OnStop: func(ctx context.Context) error {
			sender.Stop()
			engine.Stop()
			adapter.Disconnect()
			srv.Stop(ctx)
			if err := lk.Release(); err != nil {
				logger.Warn("error releasing lock", zap.Error(err))
			}
			logger.Info("daemon stopped")
			return nil
		},
	})
}
