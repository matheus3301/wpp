package store

import (
	"fmt"
	"time"
)

// QueueOutbox adds a message to the send outbox only (without creating a message row).
func (db *DB) QueueOutbox(clientMsgID, chatJID, body string) error {
	now := time.Now().UnixMilli()
	_, err := db.Exec(`
		INSERT INTO outbox (client_msg_id, chat_jid, body, status, created_at, updated_at)
		VALUES (?, ?, ?, 'queued', ?, ?)`,
		clientMsgID, chatJID, body, now, now)
	return err
}

// QueueOutboxWithMessage atomically inserts into both outbox and messages tables.
// The message is immediately visible in the TUI with status 'queued'.
func (db *DB) QueueOutboxWithMessage(clientMsgID, chatJID, body string) error {
	now := time.Now().UnixMilli()
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`
		INSERT INTO outbox (client_msg_id, chat_jid, body, status, created_at, updated_at)
		VALUES (?, ?, ?, 'queued', ?, ?)`,
		clientMsgID, chatJID, body, now, now); err != nil {
		return fmt.Errorf("insert outbox: %w", err)
	}

	if _, err := tx.Exec(`
		INSERT INTO messages (chat_jid, msg_id, sender_jid, sender_name, body, message_type, from_me, status, timestamp, created_at)
		VALUES (?, ?, '', '', ?, 'text', 1, 'queued', ?, ?)
		ON CONFLICT(chat_jid, msg_id) DO UPDATE SET
			body = excluded.body,
			status = excluded.status`,
		chatJID, clientMsgID, body, now, now); err != nil {
		return fmt.Errorf("insert message: %w", err)
	}

	return tx.Commit()
}

// MarkOutboxSending updates an outbox entry to 'sending' status.
func (db *DB) MarkOutboxSending(clientMsgID string) error {
	now := time.Now().UnixMilli()
	_, err := db.Exec(`UPDATE outbox SET status = 'sending', updated_at = ? WHERE client_msg_id = ?`, now, clientMsgID)
	return err
}

// MarkOutboxSent updates an outbox entry to 'sent' with the server message ID.
func (db *DB) MarkOutboxSent(clientMsgID, serverMsgID string) error {
	now := time.Now().UnixMilli()
	_, err := db.Exec(`UPDATE outbox SET status = 'sent', server_msg_id = ?, updated_at = ? WHERE client_msg_id = ?`, serverMsgID, now, clientMsgID)
	return err
}

// MarkOutboxFailed updates an outbox entry to 'failed' with an error message.
func (db *DB) MarkOutboxFailed(clientMsgID, errMsg string) error {
	now := time.Now().UnixMilli()
	_, err := db.Exec(`UPDATE outbox SET status = 'failed', error_message = ?, updated_at = ? WHERE client_msg_id = ?`, errMsg, now, clientMsgID)
	return err
}

// PendingOutbox returns outbox entries that are still queued.
func (db *DB) PendingOutbox() ([]OutboxEntry, error) {
	rows, err := db.Query(`
		SELECT id, client_msg_id, chat_jid, body, status, error_message, server_msg_id
		FROM outbox WHERE status = 'queued' ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var entries []OutboxEntry
	for rows.Next() {
		var e OutboxEntry
		if err := rows.Scan(&e.ID, &e.ClientMsgID, &e.ChatJID, &e.Body, &e.Status, &e.ErrorMessage, &e.ServerMsgID); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// RecoverOutbox resets any 'sending' entries back to 'queued' so they are retried.
// Call on daemon startup to reclaim in-flight messages from a previous crash.
func (db *DB) RecoverOutbox() (int64, error) {
	result, err := db.Exec(`UPDATE outbox SET status = 'queued' WHERE status = 'sending'`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
