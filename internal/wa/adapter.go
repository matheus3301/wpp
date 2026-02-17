package wa

import (
	"context"
	"fmt"

	"github.com/matheus3301/wpp/internal/bus"
	"github.com/matheus3301/wpp/internal/session"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.uber.org/zap"

	_ "github.com/mattn/go-sqlite3"
)

// Adapter wraps the whatsmeow client and manages the WhatsApp connection.
type Adapter struct {
	client    *whatsmeow.Client
	container *sqlstore.Container
	bus       *bus.Bus
	logger    *zap.Logger
	session   string
}

// NewAdapter creates a new WhatsApp adapter for the given session.
func NewAdapter(ctx context.Context, sessionName string, b *bus.Bus, logger *zap.Logger) (*Adapter, error) {
	dbPath := session.SessionDBPath(sessionName)

	container, err := sqlstore.New(ctx, "sqlite3",
		fmt.Sprintf("file:%s?_foreign_keys=on", dbPath),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("create session store: %w", err)
	}

	deviceStore, err := container.GetFirstDevice(ctx)
	if err != nil {
		return nil, fmt.Errorf("get device store: %w", err)
	}

	client := whatsmeow.NewClient(deviceStore, nil)

	return &Adapter{
		client:    client,
		container: container,
		bus:       b,
		logger:    logger,
		session:   sessionName,
	}, nil
}

// Client returns the underlying whatsmeow client.
func (a *Adapter) Client() *whatsmeow.Client {
	return a.client
}

// IsLoggedIn returns whether the adapter has valid credentials.
func (a *Adapter) IsLoggedIn() bool {
	return a.client.Store.ID != nil
}

// Connect initiates the WhatsApp connection.
func (a *Adapter) Connect() error {
	a.logger.Info("connecting to WhatsApp")
	return a.client.Connect()
}

// Disconnect terminates the WhatsApp connection.
func (a *Adapter) Disconnect() {
	a.logger.Info("disconnecting from WhatsApp")
	a.client.Disconnect()
}

// Logout invalidates the session and removes credentials.
func (a *Adapter) Logout(ctx context.Context) error {
	return a.client.Logout(ctx)
}

// RegisterEventHandler adds a handler for whatsmeow events.
func (a *Adapter) RegisterEventHandler(handler whatsmeow.EventHandler) {
	a.client.AddEventHandler(handler)
}

// GetQRChannel returns the QR channel for pairing. Must be called before Connect.
func (a *Adapter) GetQRChannel(ctx context.Context) (<-chan whatsmeow.QRChannelItem, error) {
	if a.IsLoggedIn() {
		return nil, fmt.Errorf("already logged in")
	}
	ch, err := a.client.GetQRChannel(ctx)
	if err != nil {
		return nil, fmt.Errorf("get QR channel: %w", err)
	}
	return ch, nil
}
