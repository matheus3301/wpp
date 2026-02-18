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
// Sender names are resolved via LEFT JOIN to contacts table.
func (db *DB) ListMessages(chatJID string, beforeTs int64, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 50
	}
	if beforeTs <= 0 {
		beforeTs = time.Now().UnixMilli() + 1
	}
	rows, err := db.Query(`
		SELECT m.id, m.chat_jid, m.msg_id, m.sender_jid,
			COALESCE(NULLIF(m.sender_name,''), NULLIF(ct.push_name,''), NULLIF(ct.name,''), m.sender_jid) AS display_name,
			m.body, m.message_type, m.from_me, m.status, m.timestamp
		FROM messages m
		LEFT JOIN contacts ct ON m.sender_jid = ct.jid
		WHERE m.chat_jid = ? AND m.timestamp < ?
		ORDER BY m.timestamp DESC
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
