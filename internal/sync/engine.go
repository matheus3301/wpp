package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/matheus3301/wpp/internal/bus"
	"github.com/matheus3301/wpp/internal/store"
	"go.uber.org/zap"
)

// Engine handles idempotent ingestion of messages into the store.
// It subscribes to "wa.*" events on the bus and processes them.
type Engine struct {
	db     *store.DB
	bus    *bus.Bus
	logger *zap.Logger
	cancel context.CancelFunc
}

// NewEngine creates a new sync engine.
func NewEngine(db *store.DB, b *bus.Bus, logger *zap.Logger) *Engine {
	return &Engine{
		db:     db,
		bus:    b,
		logger: logger,
	}
}

// Start subscribes to inbound WhatsApp events on the bus.
func (e *Engine) Start(ctx context.Context) {
	ctx, e.cancel = context.WithCancel(ctx)
	ch, unsub := e.bus.Subscribe("wa.", 256)

	go func() {
		defer unsub()
		for {
			select {
			case evt := <-ch:
				e.handleEvent(evt)
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Stop stops the engine.
func (e *Engine) Stop() {
	if e.cancel != nil {
		e.cancel()
	}
}

func (e *Engine) handleEvent(evt bus.Event) {
	switch evt.Kind {
	case "wa.message":
		msg, ok := evt.Payload.(*store.Message)
		if !ok {
			return
		}
		if err := e.IngestMessage(msg); err != nil {
			e.logger.Error("failed to ingest message", zap.Error(err), zap.String("msg_id", msg.MsgID))
		}
	case "wa.history_batch":
		msgs, ok := evt.Payload.([]*store.Message)
		if !ok {
			return
		}
		if err := e.IngestHistoryBatch(msgs); err != nil {
			e.logger.Error("failed to ingest history batch", zap.Error(err), zap.Int("count", len(msgs)))
		} else {
			e.logger.Info("history batch ingested", zap.Int("messages", len(msgs)))
		}
	}
}

// IngestMessage processes a single message into the store (idempotent).
func (e *Engine) IngestMessage(msg *store.Message) error {
	if err := e.db.UpsertChat(&store.Chat{
		JID:                msg.ChatJID,
		LastMessageAt:      msg.Timestamp,
		LastMessagePreview: truncate(msg.Body, 100),
	}); err != nil {
		return fmt.Errorf("upsert chat: %w", err)
	}

	if err := e.db.UpsertMessage(msg); err != nil {
		return fmt.Errorf("upsert message: %w", err)
	}

	e.bus.Publish(bus.Event{
		Kind:      "message.upserted",
		Timestamp: time.Now(),
		Payload: map[string]string{
			"chat_jid": msg.ChatJID,
			"msg_id":   msg.MsgID,
		},
	})

	return nil
}

// IngestHistoryBatch processes a batch of history messages in a transaction.
func (e *Engine) IngestHistoryBatch(msgs []*store.Message) error {
	tx, err := e.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	chatsCount := 0
	msgsCount := 0

	for _, sm := range msgs {
		if _, err := tx.Exec(`
			INSERT INTO chats (jid, last_message_at, last_message_preview, updated_at)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(jid) DO UPDATE SET
				last_message_at = MAX(chats.last_message_at, excluded.last_message_at),
				last_message_preview = CASE WHEN excluded.last_message_at > chats.last_message_at THEN excluded.last_message_preview ELSE chats.last_message_preview END,
				updated_at = excluded.updated_at`,
			sm.ChatJID, sm.Timestamp, truncate(sm.Body, 100), time.Now().UnixMilli()); err != nil {
			return fmt.Errorf("upsert chat in batch: %w", err)
		}
		chatsCount++

		if _, err := tx.Exec(`
			INSERT INTO messages (chat_jid, msg_id, sender_jid, sender_name, body, message_type, from_me, status, timestamp, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(chat_jid, msg_id) DO UPDATE SET
				sender_name = excluded.sender_name,
				body = excluded.body,
				status = excluded.status`,
			sm.ChatJID, sm.MsgID, sm.SenderJID, sm.SenderName, sm.Body, sm.MessageType, sm.FromMe, sm.Status, sm.Timestamp, time.Now().UnixMilli()); err != nil {
			return fmt.Errorf("upsert message in batch: %w", err)
		}
		msgsCount++
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit batch: %w", err)
	}

	e.bus.Publish(bus.Event{
		Kind:      "sync.history_batch",
		Timestamp: time.Now(),
		Payload: map[string]int{
			"messages_count": msgsCount,
			"chats_count":    chatsCount,
		},
	})

	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
