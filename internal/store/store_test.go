package store

import (
	"path/filepath"
	"testing"
)

func testDB(t *testing.T) *DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestMigrateAppliesOnFreshDB(t *testing.T) {
	db := testDB(t)

	// Verify the first migration set Changed=true.
	// (testDB already ran Migrate, so run it again to check idempotency.)
	result, err := db.Migrate()
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed {
		t.Error("second Migrate() should report Changed=false")
	}
	if result.Version != 2 {
		t.Errorf("version = %d, want 2 (init + fts)", result.Version)
	}
}

// TestMigrateSchemaHasRequiredColumns verifies the migration creates all
// columns the ingestion engine depends on. Regression: a stale DB created
// before migrations existed had a different schema (e.g. 'title' instead of
// 'name'), causing "table chats has no column named name" errors.
func TestMigrateSchemaHasRequiredColumns(t *testing.T) {
	db := testDB(t)

	// These columns must exist for ingestion to work.
	requiredOps := []struct {
		desc  string
		query string
		args  []any
	}{
		{"upsert chat with name", "INSERT INTO chats (jid, name, is_group, unread_count, last_message_at, last_message_preview) VALUES (?, ?, ?, ?, ?, ?)", []any{"c@s", "Test", false, 0, 1000, "hi"}},
		{"upsert message", "INSERT INTO messages (chat_jid, msg_id, sender_jid, sender_name, body, message_type, from_me, status, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)", []any{"c@s", "m1", "s@s", "Sender", "hello", "text", false, "received", 1000}},
		{"upsert contact", "INSERT INTO contacts (jid, name, push_name) VALUES (?, ?, ?)", []any{"j@s", "Name", "Push"}},
		{"queue outbox", "INSERT INTO outbox (client_msg_id, chat_jid, body, status) VALUES (?, ?, ?, ?)", []any{"cid", "c@s", "text", "queued"}},
		{"set sync state", "INSERT INTO sync_state (key, value) VALUES (?, ?)", []any{"k", "v"}},
	}

	for _, op := range requiredOps {
		t.Run(op.desc, func(t *testing.T) {
			if _, err := db.Exec(op.query, op.args...); err != nil {
				t.Fatalf("%s failed: %v", op.desc, err)
			}
		})
	}

	// Verify FTS5 works.
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM messages_fts WHERE messages_fts MATCH 'hello'").Scan(&count)
	if err != nil {
		t.Fatalf("FTS5 query failed: %v", err)
	}
	if count != 1 {
		t.Errorf("FTS5 count = %d, want 1", count)
	}
}

func TestChatUpsertAndList(t *testing.T) {
	db := testDB(t)

	chat := &Chat{JID: "123@s.whatsapp.net", Name: "Alice", LastMessageAt: 1000, LastMessagePreview: "hello"}
	if err := db.UpsertChat(chat); err != nil {
		t.Fatal(err)
	}

	// Update name.
	chat.Name = "Alice Updated"
	if err := db.UpsertChat(chat); err != nil {
		t.Fatal(err)
	}

	chats, err := db.ListChats(10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) != 1 {
		t.Fatalf("got %d chats, want 1", len(chats))
	}
	if chats[0].Name != "Alice Updated" {
		t.Errorf("name = %q, want Alice Updated", chats[0].Name)
	}
}

func TestGetChat(t *testing.T) {
	db := testDB(t)

	if err := db.UpsertChat(&Chat{JID: "a@s", Name: "A"}); err != nil {
		t.Fatal(err)
	}
	c, err := db.GetChat("a@s")
	if err != nil {
		t.Fatal(err)
	}
	if c == nil || c.Name != "A" {
		t.Errorf("got %v, want A", c)
	}

	// Non-existent.
	c, err = db.GetChat("missing@s")
	if err != nil {
		t.Fatal(err)
	}
	if c != nil {
		t.Errorf("expected nil for missing chat")
	}
}

func TestMessageUpsertIdempotent(t *testing.T) {
	db := testDB(t)

	if err := db.UpsertChat(&Chat{JID: "chat@s"}); err != nil {
		t.Fatal(err)
	}

	msg := &Message{ChatJID: "chat@s", MsgID: "msg1", Body: "hello", MessageType: "text", Timestamp: 1000}
	if err := db.UpsertMessage(msg); err != nil {
		t.Fatal(err)
	}
	// Upsert again should not create duplicate.
	msg.Body = "hello updated"
	if err := db.UpsertMessage(msg); err != nil {
		t.Fatal(err)
	}

	msgs, err := db.ListMessages("chat@s", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1 (idempotent upsert failed)", len(msgs))
	}
	if msgs[0].Body != "hello updated" {
		t.Errorf("body = %q, want hello updated", msgs[0].Body)
	}
}

func TestSearchMessages(t *testing.T) {
	db := testDB(t)

	if err := db.UpsertChat(&Chat{JID: "chat@s"}); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertMessage(&Message{ChatJID: "chat@s", MsgID: "m1", Body: "hello world", MessageType: "text", Timestamp: 1000}); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertMessage(&Message{ChatJID: "chat@s", MsgID: "m2", Body: "goodbye world", MessageType: "text", Timestamp: 2000}); err != nil {
		t.Fatal(err)
	}

	results, err := db.SearchMessages("hello", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Message.MsgID != "m1" {
		t.Errorf("msg_id = %q, want m1", results[0].Message.MsgID)
	}
}

func TestOutbox(t *testing.T) {
	db := testDB(t)

	if err := db.UpsertChat(&Chat{JID: "chat@s"}); err != nil {
		t.Fatal(err)
	}
	if err := db.QueueOutbox("client1", "chat@s", "test msg"); err != nil {
		t.Fatal(err)
	}

	pending, err := db.PendingOutbox()
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 {
		t.Fatalf("got %d pending, want 1", len(pending))
	}
	if pending[0].ClientMsgID != "client1" {
		t.Errorf("client_msg_id = %q, want client1", pending[0].ClientMsgID)
	}

	if err := db.MarkOutboxSending("client1"); err != nil {
		t.Fatal(err)
	}
	if err := db.MarkOutboxSent("client1", "server1"); err != nil {
		t.Fatal(err)
	}

	pending, err = db.PendingOutbox()
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Errorf("got %d pending after sent, want 0", len(pending))
	}
}

func TestContact(t *testing.T) {
	db := testDB(t)

	if err := db.UpsertContact(&Contact{JID: "j@s", Name: "John", PushName: "Johnny"}); err != nil {
		t.Fatal(err)
	}
	c, err := db.GetContact("j@s")
	if err != nil {
		t.Fatal(err)
	}
	if c == nil || c.PushName != "Johnny" {
		t.Errorf("got %v, want Johnny", c)
	}
}
