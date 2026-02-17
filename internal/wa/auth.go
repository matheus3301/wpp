package wa

import (
	"context"
	"time"

	"github.com/matheus3301/wpp/internal/bus"
	"go.mau.fi/whatsmeow"
)

// AuthEventType enumerates auth event types.
type AuthEventType string

const (
	AuthEventQRCode        AuthEventType = "qr_code"
	AuthEventAuthenticated AuthEventType = "authenticated"
	AuthEventAuthFailed    AuthEventType = "auth_failed"
	AuthEventTimeout       AuthEventType = "timeout"
)

// AuthEvent represents an auth lifecycle event.
type AuthEvent struct {
	Type    AuthEventType
	QRCode  string
	Message string
}

// StartQRAuth begins the QR auth flow and streams events to the bus.
// Returns a channel of AuthEvents. The caller should read until the channel closes.
func (a *Adapter) StartQRAuth(ctx context.Context) (<-chan AuthEvent, error) {
	qrChan, err := a.GetQRChannel(ctx)
	if err != nil {
		return nil, err
	}

	out := make(chan AuthEvent, 10)

	go func() {
		defer close(out)

		// Connect must be called after GetQRChannel.
		if err := a.Connect(); err != nil {
			out <- AuthEvent{Type: AuthEventAuthFailed, Message: err.Error()}
			a.bus.Publish(bus.Event{
				Kind:      "session.auth_failed",
				Timestamp: time.Now(),
				Payload:   err.Error(),
			})
			return
		}

		for item := range qrChan {
			switch item.Event {
			case "code":
				evt := AuthEvent{Type: AuthEventQRCode, QRCode: item.Code}
				out <- evt
				a.bus.Publish(bus.Event{
					Kind:      "session.qr_generated",
					Timestamp: time.Now(),
					Payload:   item.Code,
				})
			case "success":
				evt := AuthEvent{Type: AuthEventAuthenticated, Message: "authenticated"}
				out <- evt
				a.bus.Publish(bus.Event{
					Kind:      "session.authenticated",
					Timestamp: time.Now(),
				})
				return
			case "timeout":
				evt := AuthEvent{Type: AuthEventTimeout, Message: "QR code timeout"}
				out <- evt
				a.bus.Publish(bus.Event{
					Kind:      "session.auth_failed",
					Timestamp: time.Now(),
					Payload:   "timeout",
				})
				return
			default:
				if item.Error != nil {
					evt := AuthEvent{Type: AuthEventAuthFailed, Message: item.Error.Error()}
					out <- evt
					a.bus.Publish(bus.Event{
						Kind:      "session.auth_failed",
						Timestamp: time.Now(),
						Payload:   item.Error.Error(),
					})
					return
				}
			}
		}
	}()

	return out, nil
}

// IsQREvent checks whether a QR channel item is a QR code event.
func IsQREvent(item whatsmeow.QRChannelItem) bool {
	return item.Event == "code"
}
