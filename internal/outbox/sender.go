package outbox

import (
	"context"
	"time"

	"github.com/matheus3301/wpp/internal/bus"
	"github.com/matheus3301/wpp/internal/status"
	"github.com/matheus3301/wpp/internal/store"
	"go.uber.org/zap"
)

// TextSender is the interface for sending text messages via WhatsApp.
type TextSender interface {
	SendText(ctx context.Context, jid string, text string) (serverMsgID string, err error)
}

// Sender drains the outbox and sends messages via the WhatsApp adapter.
type Sender struct {
	db      *store.DB
	sender  TextSender
	bus     *bus.Bus
	machine *status.Machine
	logger  *zap.Logger
	cancel  context.CancelFunc
}

// NewSender creates a new outbox sender.
func NewSender(db *store.DB, sender TextSender, b *bus.Bus, machine *status.Machine, logger *zap.Logger) *Sender {
	return &Sender{
		db:      db,
		sender:  sender,
		bus:     b,
		machine: machine,
		logger:  logger,
	}
}

// Start begins polling the outbox for pending messages.
func (s *Sender) Start(ctx context.Context) {
	ctx, s.cancel = context.WithCancel(ctx)
	go s.loop(ctx)
}

// Stop stops the sender loop.
func (s *Sender) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *Sender) loop(ctx context.Context) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.processPending(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (s *Sender) processPending(ctx context.Context) {
	// Skip sending when not connected (2.3).
	if s.machine != nil {
		current := s.machine.Current()
		if current != status.Ready {
			return
		}
	}

	pending, err := s.db.PendingOutbox()
	if err != nil {
		s.logger.Error("failed to read outbox", zap.Error(err))
		return
	}

	for _, entry := range pending {
		if err := s.db.MarkOutboxSending(entry.ClientMsgID); err != nil {
			s.logger.Error("failed to mark sending", zap.Error(err), zap.String("client_msg_id", entry.ClientMsgID))
			continue
		}

		// Update the existing message row to 'sending' status.
		_ = s.db.UpsertMessage(&store.Message{
			ChatJID:     entry.ChatJID,
			MsgID:       entry.ClientMsgID,
			Body:        entry.Body,
			MessageType: "text",
			FromMe:      true,
			Status:      "sending",
			Timestamp:   time.Now().UnixMilli(),
		})
		s.bus.Publish(bus.Event{
			Kind:      "message.upserted",
			Timestamp: time.Now(),
			Payload:   map[string]string{"chat_jid": entry.ChatJID, "msg_id": entry.ClientMsgID},
		})

		serverMsgID, err := s.sender.SendText(ctx, entry.ChatJID, entry.Body)
		if err != nil {
			s.logger.Error("failed to send message", zap.Error(err), zap.String("client_msg_id", entry.ClientMsgID))
			_ = s.db.MarkOutboxFailed(entry.ClientMsgID, err.Error())
			_ = s.db.UpsertMessage(&store.Message{
				ChatJID: entry.ChatJID, MsgID: entry.ClientMsgID,
				Body: entry.Body, MessageType: "text", FromMe: true,
				Status: "failed", Timestamp: time.Now().UnixMilli(),
			})
			s.bus.Publish(bus.Event{
				Kind:      "message.send_failed",
				Timestamp: time.Now(),
				Payload: map[string]string{
					"client_msg_id": entry.ClientMsgID,
					"error":         err.Error(),
				},
			})
			continue
		}

		if err := s.db.MarkOutboxSent(entry.ClientMsgID, serverMsgID); err != nil {
			s.logger.Error("failed to mark sent", zap.Error(err), zap.String("client_msg_id", entry.ClientMsgID))
		}

		_ = s.db.UpsertMessage(&store.Message{
			ChatJID: entry.ChatJID, MsgID: entry.ClientMsgID,
			Body: entry.Body, MessageType: "text", FromMe: true,
			Status: "sent", Timestamp: time.Now().UnixMilli(),
		})

		s.logger.Info("message sent", zap.String("client_msg_id", entry.ClientMsgID), zap.String("server_msg_id", serverMsgID))
		s.bus.Publish(bus.Event{
			Kind:      "message.send_ack",
			Timestamp: time.Now(),
			Payload: map[string]string{
				"client_msg_id": entry.ClientMsgID,
				"server_msg_id": serverMsgID,
			},
		})
	}
}
