package store

import "fmt"

// LIDMapping maps a LID JID to a phone number JID.
type LIDMapping struct {
	LID string
	PN  string
}

// SyncLIDMap replaces the lid_map table with the given mappings.
func (db *DB) SyncLIDMap(mappings []LIDMapping) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM lid_map`); err != nil {
		return fmt.Errorf("clear lid_map: %w", err)
	}

	for _, m := range mappings {
		if _, err := tx.Exec(`INSERT INTO lid_map (lid, pn) VALUES (?, ?)`, m.LID, m.PN); err != nil {
			return fmt.Errorf("insert lid_map %q: %w", m.LID, err)
		}
	}
	return tx.Commit()
}

// ReconcileLIDs merges LID chat entries into their phone number equivalents.
// For each LID chat that has a known PN mapping:
// 1. Reassigns messages from the LID chat to the PN chat
// 2. Reassigns contacts from the LID to the PN
// 3. Deletes the LID chat entry
// Returns the number of LID chats merged.
func (db *DB) ReconcileLIDs() (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Ensure PN chats exist for all mapped LID chats.
	if _, err := tx.Exec(`
		INSERT INTO chats (jid, name, is_group, unread_count, last_message_at, last_message_preview, updated_at)
		SELECT lm.pn || '@s.whatsapp.net', c.name, c.is_group, c.unread_count, c.last_message_at, c.last_message_preview, c.updated_at
		FROM chats c
		JOIN lid_map lm ON c.jid = lm.lid || '@lid'
		WHERE NOT EXISTS (SELECT 1 FROM chats WHERE jid = lm.pn || '@s.whatsapp.net')
		ON CONFLICT(jid) DO UPDATE SET
			last_message_at = MAX(chats.last_message_at, excluded.last_message_at),
			last_message_preview = CASE WHEN excluded.last_message_at > chats.last_message_at THEN excluded.last_message_preview ELSE chats.last_message_preview END,
			name = CASE WHEN chats.name = '' THEN excluded.name ELSE chats.name END,
			updated_at = excluded.updated_at
	`); err != nil {
		return 0, fmt.Errorf("ensure PN chats: %w", err)
	}

	// Reassign messages from LID chats to PN chats.
	if _, err := tx.Exec(`
		UPDATE messages SET
			chat_jid = (SELECT lm.pn || '@s.whatsapp.net' FROM lid_map lm WHERE messages.chat_jid = lm.lid || '@lid'),
			sender_jid = COALESCE(
				(SELECT lm2.pn || '@s.whatsapp.net' FROM lid_map lm2 WHERE messages.sender_jid = lm2.lid || '@lid'),
				messages.sender_jid
			)
		WHERE chat_jid IN (SELECT lm.lid || '@lid' FROM lid_map lm)
	`); err != nil {
		return 0, fmt.Errorf("reassign messages: %w", err)
	}

	// Reassign contacts from LID to PN.
	if _, err := tx.Exec(`
		INSERT INTO contacts (jid, name, push_name, updated_at)
		SELECT lm.pn || '@s.whatsapp.net', ct.name, ct.push_name, ct.updated_at
		FROM contacts ct
		JOIN lid_map lm ON ct.jid = lm.lid || '@lid'
		ON CONFLICT(jid) DO UPDATE SET
			name = CASE WHEN contacts.name = '' AND excluded.name != '' THEN excluded.name ELSE contacts.name END,
			push_name = CASE WHEN contacts.push_name = '' AND excluded.push_name != '' THEN excluded.push_name ELSE contacts.push_name END,
			updated_at = excluded.updated_at
	`); err != nil {
		return 0, fmt.Errorf("reassign contacts: %w", err)
	}

	// Delete LID contacts.
	if _, err := tx.Exec(`
		DELETE FROM contacts WHERE jid IN (SELECT lm.lid || '@lid' FROM lid_map lm)
	`); err != nil {
		return 0, fmt.Errorf("delete LID contacts: %w", err)
	}

	// Delete LID chats.
	result, err := tx.Exec(`
		DELETE FROM chats WHERE jid IN (SELECT lm.lid || '@lid' FROM lid_map lm)
	`)
	if err != nil {
		return 0, fmt.Errorf("delete LID chats: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}

	return result.RowsAffected()
}
