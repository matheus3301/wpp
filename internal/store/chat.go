package store

import (
	"database/sql"
	"time"
)

// UpsertChat inserts or updates a chat record.
func (db *DB) UpsertChat(c *Chat) error {
	now := time.Now().UnixMilli()
	_, err := db.Exec(`
		INSERT INTO chats (jid, name, is_group, unread_count, last_message_at, last_message_preview, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(jid) DO UPDATE SET
			name = excluded.name,
			is_group = excluded.is_group,
			unread_count = excluded.unread_count,
			last_message_at = excluded.last_message_at,
			last_message_preview = excluded.last_message_preview,
			updated_at = excluded.updated_at`,
		c.JID, c.Name, c.IsGroup, c.UnreadCount, c.LastMessageAt, c.LastMessagePreview, now)
	return err
}

// ListChats returns chats sorted by last message timestamp descending.
// Names are resolved via LEFT JOIN to contacts table with fallback:
// chat.name -> contact.push_name -> contact.name -> chat.jid
func (db *DB) ListChats(limit, offset int) ([]Chat, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := db.Query(`
		SELECT c.jid,
			COALESCE(NULLIF(c.name,''), NULLIF(ct.push_name,''), NULLIF(ct.name,''), c.jid) AS display_name,
			c.is_group, c.unread_count, c.last_message_at, c.last_message_preview
		FROM chats c
		LEFT JOIN contacts ct ON c.jid = ct.jid
		WHERE c.jid NOT LIKE '%@lid'
		ORDER BY c.last_message_at DESC
		LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var chats []Chat
	for rows.Next() {
		var c Chat
		if err := rows.Scan(&c.JID, &c.Name, &c.IsGroup, &c.UnreadCount, &c.LastMessageAt, &c.LastMessagePreview); err != nil {
			return nil, err
		}
		chats = append(chats, c)
	}
	return chats, rows.Err()
}

// GetChat returns a single chat by JID.
func (db *DB) GetChat(jid string) (*Chat, error) {
	var c Chat
	err := db.QueryRow(`
		SELECT c.jid,
			COALESCE(NULLIF(c.name,''), NULLIF(ct.push_name,''), NULLIF(ct.name,''), c.jid) AS display_name,
			c.is_group, c.unread_count, c.last_message_at, c.last_message_preview
		FROM chats c
		LEFT JOIN contacts ct ON c.jid = ct.jid
		WHERE c.jid = ?`, jid).
		Scan(&c.JID, &c.Name, &c.IsGroup, &c.UnreadCount, &c.LastMessageAt, &c.LastMessagePreview)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}
