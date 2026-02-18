package daemon

import (
	"context"
	"time"

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

func provideBus(logger *zap.Logger) *bus.Bus {
	return bus.NewWithLogger(logger)
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

func provideSender(db *store.DB, adapter *wa.Adapter, b *bus.Bus, m *status.Machine, logger *zap.Logger) *outbox.Sender {
	return outbox.NewSender(db, adapter, b, m, logger)
}

func provideSessionService(p Params, m *status.Machine, adapter *wa.Adapter, b *bus.Bus, db *store.DB) *api.SessionService {
	return api.NewSessionService(p.SessionName, m, adapter, b, db)
}

func provideSyncService(p Params, adapter *wa.Adapter, b *bus.Bus, m *status.Machine) *api.SyncService {
	return api.NewSyncService(adapter, b, m, p.SessionName)
}

func provideChatService(p Params, db *store.DB, b *bus.Bus) *api.ChatService {
	return api.NewChatService(db, b, p.SessionName)
}

func provideMessageService(p Params, db *store.DB, b *bus.Bus) *api.MessageService {
	return api.NewMessageService(db, b, p.SessionName)
}

func registerLifecycle(lc fx.Lifecycle, srv *Server, lk *lock.Lock, db *store.DB, adapter *wa.Adapter, engine *intsync.Engine, sender *outbox.Sender, machine *status.Machine, b *bus.Bus, logger *zap.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			// Recover any in-flight outbox messages from a previous crash.
			if recovered, err := db.RecoverOutbox(); err != nil {
				logger.Warn("outbox recovery failed", zap.Error(err))
			} else if recovered > 0 {
				logger.Info("recovered outbox entries", zap.Int64("count", recovered))
			}

			// Start sync engine (subscribes to wa.* bus events).
			engine.Start(context.Background())

			// Register event handler for whatsmeow events.
			handler := wa.NewEventHandler(b, machine, adapter, logger)
			adapter.RegisterEventHandler(handler.Handle)

			// Start gRPC server in background.
			go func() {
				if err := srv.Start(); err != nil {
					logger.Error("gRPC server error", zap.Error(err))
				}
			}()

			// Start outbox sender.
			sender.Start(context.Background())

			// Listen for sync.connected to trigger LID reconciliation.
			go func() {
				ch, unsub := b.Subscribe("sync.connected", 1)
				defer unsub()

				select {
				case <-ch:
					// Wait briefly for LID map to be populated by whatsmeow.
					time.Sleep(3 * time.Second)

					mappings := adapter.GetLIDMappings(context.Background())
					if len(mappings) > 0 {
						if err := db.SyncLIDMap(mappings); err != nil {
							logger.Error("failed to sync lid map", zap.Error(err))
							return
						}
						merged, err := db.ReconcileLIDs()
						if err != nil {
							logger.Error("failed to reconcile LIDs", zap.Error(err))
						} else if merged > 0 {
							logger.Info("reconciled LID chats", zap.Int64("merged", merged))
							b.Publish(bus.Event{Kind: "message.upserted", Timestamp: time.Now()})
						}
					}
				case <-context.Background().Done():
				}
			}()

			// Transition state based on auth status.
			if adapter.IsLoggedIn() {
				_ = machine.Transition(status.Connecting)
				go func() {
					connectCtx, connectCancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer connectCancel()
					done := make(chan error, 1)
					go func() { done <- adapter.Connect() }()
					select {
					case err := <-done:
						if err != nil {
							logger.Error("auto-connect failed", zap.Error(err))
							_ = machine.Transition(status.Error)
						}
					case <-connectCtx.Done():
						logger.Error("auto-connect timed out")
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
			if err := db.Close(); err != nil {
				logger.Warn("error closing database", zap.Error(err))
			}
			if err := lk.Release(); err != nil {
				logger.Warn("error releasing lock", zap.Error(err))
			}
			logger.Info("daemon stopped")
			return nil
		},
	})
}
