package wa

import (
	"context"
	"fmt"

	"github.com/matheus3301/wpp/internal/bus"
	"github.com/matheus3301/wpp/internal/session"
	"github.com/matheus3301/wpp/internal/store"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	wastore "go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

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
	// Set device name shown on the phone's linked devices list.
	wastore.SetOSInfo("WPP-TUI", [3]uint32{0, 1, 0})

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

// SendText sends a text message to the given JID. Returns the server message ID.
func (a *Adapter) SendText(ctx context.Context, jid string, text string) (string, error) {
	to, err := types.ParseJID(jid)
	if err != nil {
		return "", fmt.Errorf("parse JID: %w", err)
	}
	resp, err := a.client.SendMessage(ctx, to, &waE2E.Message{
		Conversation: proto.String(text),
	})
	if err != nil {
		return "", fmt.Errorf("send message: %w", err)
	}
	return resp.ID, nil
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

// GetContacts returns all contacts from the whatsmeow device store.
// Keys are JID strings, values are contact names.
func (a *Adapter) GetContacts(ctx context.Context) []store.Contact {
	allContacts, err := a.client.Store.Contacts.GetAllContacts(ctx)
	if err != nil {
		a.logger.Warn("failed to get contacts from device store", zap.Error(err))
		return nil
	}
	var contacts []store.Contact
	for jid, info := range allContacts {
		contacts = append(contacts, store.Contact{
			JID:      jid.ToNonAD().String(),
			Name:     info.FullName,
			PushName: info.PushName,
		})
	}
	return contacts
}

// PhoneNumber returns the phone number from the device store, or empty string.
func (a *Adapter) PhoneNumber() string {
	if a.client.Store.ID == nil {
		return ""
	}
	return a.client.Store.ID.User
}

// GetLIDMappings returns all LID-to-PN mappings from the whatsmeow device store.
func (a *Adapter) GetLIDMappings(ctx context.Context) []store.LIDMapping {
	if a.client == nil || a.client.Store == nil || a.client.Store.LIDs == nil {
		return nil
	}

	// Get all contacts, then resolve LIDs for each.
	// Unfortunately there's no bulk "get all LID mappings" API,
	// so we query the session.db directly via the contacts list.
	allContacts, err := a.client.Store.Contacts.GetAllContacts(ctx)
	if err != nil {
		return nil
	}

	var mappings []store.LIDMapping
	for jid := range allContacts {
		normalized := jid.ToNonAD()
		if normalized.Server == types.DefaultUserServer {
			lid, err := a.client.Store.LIDs.GetLIDForPN(ctx, normalized)
			if err == nil && !lid.IsEmpty() {
				mappings = append(mappings, store.LIDMapping{
					LID: lid.User,
					PN:  normalized.User,
				})
			}
		}
	}
	return mappings
}

// ResolveLID resolves a LID JID to its phone number JID using the device store mapping.
// Returns the original JID if it's not a LID or if resolution fails.
func (a *Adapter) ResolveLID(ctx context.Context, jid types.JID) types.JID {
	if jid.Server != types.HiddenUserServer && jid.Server != types.HostedLIDServer {
		return jid
	}
	if a.client == nil || a.client.Store == nil || a.client.Store.LIDs == nil {
		return jid
	}
	pn, err := a.client.Store.LIDs.GetPNForLID(ctx, jid)
	if err != nil || pn.IsEmpty() {
		return jid
	}
	return pn
}
