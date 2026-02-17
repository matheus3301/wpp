package store

import (
	"database/sql"
	"time"
)

// UpsertContact inserts or updates a contact.
func (db *DB) UpsertContact(c *Contact) error {
	now := time.Now().UnixMilli()
	_, err := db.Exec(`
		INSERT INTO contacts (jid, name, push_name, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(jid) DO UPDATE SET
			name = excluded.name,
			push_name = excluded.push_name,
			updated_at = excluded.updated_at`,
		c.JID, c.Name, c.PushName, now)
	return err
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
