package store

import (
	"database/sql"
	"fmt"
	"time"
)

// UpsertContact inserts or updates a contact.
func (db *DB) UpsertContact(c *Contact) error {
	now := time.Now().UnixMilli()
	_, err := db.Exec(`
		INSERT INTO contacts (jid, name, push_name, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(jid) DO UPDATE SET
			name = CASE WHEN excluded.name != '' THEN excluded.name ELSE contacts.name END,
			push_name = CASE WHEN excluded.push_name != '' THEN excluded.push_name ELSE contacts.push_name END,
			updated_at = excluded.updated_at`,
		c.JID, c.Name, c.PushName, now)
	return err
}

// BulkUpsertContacts inserts or updates multiple contacts in a single transaction.
func (db *DB) BulkUpsertContacts(contacts []Contact) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UnixMilli()
	for _, c := range contacts {
		if _, err := tx.Exec(`
			INSERT INTO contacts (jid, name, push_name, updated_at)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(jid) DO UPDATE SET
				name = CASE WHEN excluded.name != '' THEN excluded.name ELSE contacts.name END,
				push_name = CASE WHEN excluded.push_name != '' THEN excluded.push_name ELSE contacts.push_name END,
				updated_at = excluded.updated_at`,
			c.JID, c.Name, c.PushName, now); err != nil {
			return fmt.Errorf("upsert contact %q: %w", c.JID, err)
		}
	}
	return tx.Commit()
}

// GetContact returns a contact by JID.
func (db *DB) GetContact(jid string) (*Contact, error) {
	var c Contact
	err := db.QueryRow(`SELECT jid, name, push_name FROM contacts WHERE jid = ?`, jid).
		Scan(&c.JID, &c.Name, &c.PushName)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ChatCount returns the total number of chats.
func (db *DB) ChatCount() (int64, error) {
	var count int64
	err := db.QueryRow(`SELECT COUNT(*) FROM chats`).Scan(&count)
	return count, err
}

// MessageCount returns the total number of messages.
func (db *DB) MessageCount() (int64, error) {
	var count int64
	err := db.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&count)
	return count, err
}
