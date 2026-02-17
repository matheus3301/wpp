package store

import "time"

// QueueOutbox adds a message to the send outbox.
func (db *DB) QueueOutbox(clientMsgID, chatJID, body string) error {
	now := time.Now().UnixMilli()
	_, err := db.Exec(`
		INSERT INTO outbox (client_msg_id, chat_jid, body, status, created_at, updated_at)
		VALUES (?, ?, ?, 'queued', ?, ?)`,
		clientMsgID, chatJID, body, now, now)
	return err
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
