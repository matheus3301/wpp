package store

import "time"

// UpsertMessage inserts or updates a message (idempotent on chat_jid + msg_id).
func (db *DB) UpsertMessage(m *Message) error {
	now := time.Now().UnixMilli()
	_, err := db.Exec(`
		INSERT INTO messages (chat_jid, msg_id, sender_jid, sender_name, body, message_type, from_me, status, timestamp, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(chat_jid, msg_id) DO UPDATE SET
			sender_name = excluded.sender_name,
			body = excluded.body,
			status = excluded.status`,
		m.ChatJID, m.MsgID, m.SenderJID, m.SenderName, m.Body, m.MessageType, m.FromMe, m.Status, m.Timestamp, now)
	return err
}

// ListMessages returns messages for a chat using keyset pagination by timestamp.
func (db *DB) ListMessages(chatJID string, beforeTs int64, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 50
	}
	if beforeTs <= 0 {
		beforeTs = time.Now().UnixMilli() + 1
	}
	rows, err := db.Query(`
		SELECT id, chat_jid, msg_id, sender_jid, sender_name, body, message_type, from_me, status, timestamp
		FROM messages
		WHERE chat_jid = ? AND timestamp < ?
		ORDER BY timestamp DESC
		LIMIT ?`, chatJID, beforeTs, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ChatJID, &m.MsgID, &m.SenderJID, &m.SenderName, &m.Body, &m.MessageType, &m.FromMe, &m.Status, &m.Timestamp); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}
