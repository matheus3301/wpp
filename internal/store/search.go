package store

// SearchMessages performs a full-text search on message bodies.
func (db *DB) SearchMessages(query string, chatJID string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 50
	}

	q := `
		SELECT m.id, m.chat_jid, m.msg_id, m.sender_jid, m.sender_name, m.body,
		       m.message_type, m.from_me, m.status, m.timestamp,
		       snippet(messages_fts, 0, '<<', '>>', '...', 32)
		FROM messages_fts f
		JOIN messages m ON m.id = f.rowid
		WHERE messages_fts MATCH ?`

	args := []any{query}
	if chatJID != "" {
		q += " AND m.chat_jid = ?"
		args = append(args, chatJID)
	}
	q += " ORDER BY rank LIMIT ?"
	args = append(args, limit)

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(
			&r.Message.ID, &r.Message.ChatJID, &r.Message.MsgID,
			&r.Message.SenderJID, &r.Message.SenderName, &r.Message.Body,
			&r.Message.MessageType, &r.Message.FromMe, &r.Message.Status,
			&r.Message.Timestamp, &r.Snippet,
		); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}
