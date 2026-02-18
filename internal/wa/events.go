package wa

import (
	"time"

	"github.com/matheus3301/wpp/internal/bus"
	"github.com/matheus3301/wpp/internal/status"
	"github.com/matheus3301/wpp/internal/store"
	"go.mau.fi/whatsmeow/types/events"
	"go.uber.org/zap"
)

// EventHandler processes whatsmeow events, drives the state machine,
// and publishes parsed domain events on the bus. It does NOT call the
// sync engine directly — the engine subscribes to the bus independently.
type EventHandler struct {
	bus     *bus.Bus
	machine *status.Machine
	logger  *zap.Logger
}

// NewEventHandler creates a new event handler.
func NewEventHandler(b *bus.Bus, machine *status.Machine, logger *zap.Logger) *EventHandler {
	return &EventHandler{
		bus:     b,
		machine: machine,
		logger:  logger,
	}
}

// Handle is the main whatsmeow event handler function.
func (h *EventHandler) Handle(rawEvt any) {
	switch evt := rawEvt.(type) {
	case *events.Message:
		h.handleMessage(evt)
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

func (h *EventHandler) handleMessage(evt *events.Message) {
	if h.machine.Current() == status.Syncing {
		_ = h.machine.Transition(status.Ready)
	}

	parsed := ParseLiveMessage(evt)
	h.bus.Publish(bus.Event{
		Kind:      "wa.message",
		Timestamp: time.Now(),
		Payload:   parsed.ToStoreMessage(),
	})
}

func (h *EventHandler) handleHistorySync(evt *events.HistorySync) {
	data := evt.Data
	if data == nil {
		return
	}

	var msgs []*store.Message
	for _, conv := range data.GetConversations() {
		chatJID := conv.GetID()
		for _, hm := range conv.GetMessages() {
			wmsg := hm.GetMessage()
			if wmsg == nil || wmsg.GetMessage() == nil {
				continue
			}
			info := wmsg.GetMessage()
			parsed := &ParsedMessage{
				ChatJID:     chatJID,
				MsgID:       wmsg.GetKey().GetID(),
				SenderJID:   wmsg.GetKey().GetParticipant(),
				Body:        extractTextBody(info),
				MessageType: detectMessageType(info),
				FromMe:      wmsg.GetKey().GetFromMe(),
				Timestamp:   int64(wmsg.GetMessageTimestamp()) * 1000,
			}
			msgs = append(msgs, parsed.ToStoreMessage())
		}
	}

	if len(msgs) > 0 {
		h.bus.Publish(bus.Event{
			Kind:      "wa.history_batch",
			Timestamp: time.Now(),
			Payload:   msgs,
		})
	}
}
