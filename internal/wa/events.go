package wa

import (
	"context"
	"time"

	"github.com/matheus3301/wpp/internal/bus"
	"github.com/matheus3301/wpp/internal/status"
	"github.com/matheus3301/wpp/internal/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.uber.org/zap"
)

// EventHandler processes whatsmeow events, drives the state machine,
// and publishes parsed domain events on the bus. It does NOT call the
// sync engine directly — the engine subscribes to the bus independently.
type EventHandler struct {
	bus     *bus.Bus
	machine *status.Machine
	adapter *Adapter
	logger  *zap.Logger
}

// NewEventHandler creates a new event handler.
func NewEventHandler(b *bus.Bus, machine *status.Machine, adapter *Adapter, logger *zap.Logger) *EventHandler {
	return &EventHandler{
		bus:     b,
		machine: machine,
		adapter: adapter,
		logger:  logger,
	}
}

// Handle is the main whatsmeow event handler function.
func (h *EventHandler) Handle(rawEvt any) {
	switch evt := rawEvt.(type) {
	case *events.Message:
		h.handleMessage(evt)
	case *events.PushName:
		h.handlePushName(evt)
	case *events.Connected:
		h.logger.Info("WhatsApp connected")
		current := h.machine.Current()
		if current == status.AuthRequired || current == status.Reconnecting {
			_ = h.machine.Transition(status.Connecting)
		}
		_ = h.machine.Transition(status.Syncing)
		h.bus.Publish(bus.Event{Kind: "sync.connected", Timestamp: time.Now()})
	case *events.Disconnected:
		h.logger.Warn("WhatsApp disconnected")
		_ = h.machine.Transition(status.Reconnecting)
		h.bus.Publish(bus.Event{Kind: "sync.disconnected", Timestamp: time.Now()})
	case *events.HistorySync:
		h.handleHistorySync(evt)
	case *events.LoggedOut:
		h.logger.Warn("WhatsApp logged out", zap.String("reason", evt.Reason.String()))
		_ = h.machine.Transition(status.AuthRequired)
		h.bus.Publish(bus.Event{Kind: "session.logged_out", Timestamp: time.Now(), Payload: evt.Reason.String()})
	}
}

// resolveJID normalizes a JID string, resolving LIDs to phone number JIDs
// via the whatsmeow device store if the adapter is available.
func (h *EventHandler) resolveJID(jid string) string {
	if h.adapter == nil {
		return NormalizeJID(jid)
	}
	parsed, err := types.ParseJID(jid)
	if err != nil {
		return jid
	}
	resolved := h.adapter.ResolveLID(context.Background(), parsed.ToNonAD())
	return resolved.ToNonAD().String()
}

func (h *EventHandler) handleMessage(evt *events.Message) {
	if h.machine.Current() == status.Syncing {
		_ = h.machine.Transition(status.Ready)
	}

	parsed := ParseLiveMessage(evt)
	// Resolve LID JIDs to phone number JIDs.
	parsed.ChatJID = h.resolveJID(parsed.ChatJID)
	parsed.SenderJID = h.resolveJID(parsed.SenderJID)

	h.bus.Publish(bus.Event{
		Kind:      "wa.message",
		Timestamp: time.Now(),
		Payload:   parsed.ToStoreMessage(),
	})

	// Also publish contact info from the push name if available.
	if evt.Info.PushName != "" && !evt.Info.IsFromMe {
		h.bus.Publish(bus.Event{
			Kind:      "wa.contact",
			Timestamp: time.Now(),
			Payload: &store.Contact{
				JID:      h.resolveJID(evt.Info.Sender.ToNonAD().String()),
				PushName: evt.Info.PushName,
			},
		})
	}
}

func (h *EventHandler) handlePushName(evt *events.PushName) {
	h.bus.Publish(bus.Event{
		Kind:      "wa.contact",
		Timestamp: time.Now(),
		Payload: &store.Contact{
			JID:      h.resolveJID(evt.JID.ToNonAD().String()),
			PushName: evt.NewPushName,
		},
	})
}

func (h *EventHandler) handleHistorySync(evt *events.HistorySync) {
	data := evt.Data
	if data == nil {
		return
	}

	var msgs []*store.Message
	var contacts []*store.Contact
	for _, conv := range data.GetConversations() {
		chatJID := h.resolveJID(conv.GetID())

		// Extract chat/contact name from conversation metadata.
		convName := conv.GetName()
		if convName != "" {
			contacts = append(contacts, &store.Contact{
				JID:  chatJID,
				Name: convName,
			})
		}

		for _, hm := range conv.GetMessages() {
			wmsg := hm.GetMessage()
			if wmsg == nil || wmsg.GetMessage() == nil {
				continue
			}
			info := wmsg.GetMessage()
			senderJID := h.resolveJID(wmsg.GetKey().GetParticipant())
			parsed := &ParsedMessage{
				ChatJID:     chatJID,
				MsgID:       wmsg.GetKey().GetID(),
				SenderJID:   senderJID,
				Body:        extractTextBody(info),
				MessageType: detectMessageType(info),
				FromMe:      wmsg.GetKey().GetFromMe(),
				Timestamp:   int64(wmsg.GetMessageTimestamp()) * 1000,
			}
			msgs = append(msgs, parsed.ToStoreMessage())

			// Extract push name from history message if available.
			if pn := wmsg.GetPushName(); pn != "" && senderJID != "" {
				contacts = append(contacts, &store.Contact{
					JID:      senderJID,
					PushName: pn,
				})
			}
		}
	}

	if len(msgs) > 0 {
		h.bus.Publish(bus.Event{
			Kind:      "wa.history_batch",
			Timestamp: time.Now(),
			Payload:   msgs,
		})
	}

	if len(contacts) > 0 {
		h.bus.Publish(bus.Event{
			Kind:      "wa.contact_batch",
			Timestamp: time.Now(),
			Payload:   contacts,
		})
	}
}
