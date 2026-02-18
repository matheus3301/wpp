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
	if result.Version != 3 {
		t.Errorf("version = %d, want 3 (init + fts + lid_map)", result.Version)
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

// TestContactNameResolution verifies that ListMessages returns resolved sender names
// from the contacts table when the message's sender_name is empty.
func TestContactNameResolution(t *testing.T) {
	db := testDB(t)

	if err := db.UpsertChat(&Chat{JID: "chat@s"}); err != nil {
		t.Fatal(err)
	}
	// Insert message with empty sender_name.
	if err := db.UpsertMessage(&Message{
		ChatJID: "chat@s", MsgID: "m1", SenderJID: "sender@s",
		SenderName: "", Body: "hello", MessageType: "text", Timestamp: 1000,
	}); err != nil {
		t.Fatal(err)
	}

	// Before contact upsert, sender_name should fall back to sender JID.
	msgs, err := db.ListMessages("chat@s", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	if msgs[0].SenderName != "sender@s" {
		t.Errorf("before contact: sender_name = %q, want sender@s (JID fallback)", msgs[0].SenderName)
	}

	// Upsert contact with push_name.
	if err := db.UpsertContact(&Contact{JID: "sender@s", PushName: "Alice"}); err != nil {
		t.Fatal(err)
	}

	// After contact upsert, sender_name should resolve to push_name.
	msgs, err = db.ListMessages("chat@s", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if msgs[0].SenderName != "Alice" {
		t.Errorf("after contact: sender_name = %q, want Alice (push_name)", msgs[0].SenderName)
	}
}

// TestChatNameFallback verifies that ListChats resolves chat names with the
// correct fallback order: chat.name -> contact.push_name -> contact.name -> jid.
func TestChatNameFallback(t *testing.T) {
	db := testDB(t)

	// Chat with no name, no contact.
	if err := db.UpsertChat(&Chat{JID: "noname@s", LastMessageAt: 3000}); err != nil {
		t.Fatal(err)
	}
	// Chat with no name, but contact has push_name.
	if err := db.UpsertChat(&Chat{JID: "push@s", LastMessageAt: 2000}); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertContact(&Contact{JID: "push@s", PushName: "PushAlice"}); err != nil {
		t.Fatal(err)
	}
	// Chat with no name, contact has name but no push_name.
	if err := db.UpsertChat(&Chat{JID: "named@s", LastMessageAt: 1000}); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertContact(&Contact{JID: "named@s", Name: "NamedBob"}); err != nil {
		t.Fatal(err)
	}
	// Chat with its own name (should take precedence).
	if err := db.UpsertChat(&Chat{JID: "own@s", Name: "OwnName", LastMessageAt: 500}); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertContact(&Contact{JID: "own@s", PushName: "ContactPush"}); err != nil {
		t.Fatal(err)
	}

	chats, err := db.ListChats(10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) != 4 {
		t.Fatalf("got %d chats, want 4", len(chats))
	}

	// Sorted by last_message_at DESC.
	expected := map[string]string{
		"noname@s": "noname@s",  // falls back to JID
		"push@s":   "PushAlice", // falls back to contact.push_name
		"named@s":  "NamedBob",  // falls back to contact.name
		"own@s":    "OwnName",   // chat.name takes precedence
	}
	for _, c := range chats {
		want, ok := expected[c.JID]
		if !ok {
			t.Errorf("unexpected chat JID: %s", c.JID)
			continue
		}
		if c.Name != want {
			t.Errorf("chat %s: name = %q, want %q", c.JID, c.Name, want)
		}
	}
}

// TestBulkUpsertContacts verifies bulk insert of contacts.
func TestBulkUpsertContacts(t *testing.T) {
	db := testDB(t)

	contacts := []Contact{
		{JID: "a@s", Name: "Alice", PushName: "Ali"},
		{JID: "b@s", Name: "Bob", PushName: "Bobby"},
	}
	if err := db.BulkUpsertContacts(contacts); err != nil {
		t.Fatal(err)
	}

	c, err := db.GetContact("a@s")
	if err != nil {
		t.Fatal(err)
	}
	if c == nil || c.Name != "Alice" {
		t.Errorf("got %v, want Alice", c)
	}

	c, err = db.GetContact("b@s")
	if err != nil {
		t.Fatal(err)
	}
	if c == nil || c.PushName != "Bobby" {
		t.Errorf("got %v, want Bobby", c)
	}
}

// TestReconcileLIDs verifies that LID chats are merged into their PN equivalents.
// Regression: WhatsApp history sync creates chats with LID JIDs (e.g. "3917077286968@lid")
// that are duplicates of phone number JIDs (e.g. "558592403672@s.whatsapp.net").
func TestReconcileLIDs(t *testing.T) {
	db := testDB(t)

	// Create a PN chat with some messages.
	if err := db.UpsertChat(&Chat{JID: "558592403672@s.whatsapp.net", Name: "", LastMessageAt: 1000, LastMessagePreview: "pn msg"}); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertMessage(&Message{ChatJID: "558592403672@s.whatsapp.net", MsgID: "pn1", Body: "from pn", MessageType: "text", Timestamp: 1000}); err != nil {
		t.Fatal(err)
	}

	// Create a LID chat (duplicate of the same user) with messages and contact.
	if err := db.UpsertChat(&Chat{JID: "3917077286968@lid", Name: "", LastMessageAt: 2000, LastMessagePreview: "lid msg"}); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertMessage(&Message{ChatJID: "3917077286968@lid", MsgID: "lid1", SenderJID: "3917077286968@lid", Body: "from lid", MessageType: "text", Timestamp: 2000}); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertContact(&Contact{JID: "3917077286968@lid", PushName: "Eric"}); err != nil {
		t.Fatal(err)
	}

	// Verify both chats exist before reconciliation.
	chats, err := db.ListChats(100, 0)
	if err != nil {
		t.Fatal(err)
	}
	// ListChats filters @lid, so only PN chat should be visible.
	pnFound := false
	for _, c := range chats {
		if c.JID == "558592403672@s.whatsapp.net" {
			pnFound = true
		}
		if c.JID == "3917077286968@lid" {
			t.Error("ListChats should filter out @lid chats")
		}
	}
	if !pnFound {
		t.Error("PN chat not found in ListChats")
	}

	// Sync LID map and reconcile.
	if err := db.SyncLIDMap([]LIDMapping{{LID: "3917077286968", PN: "558592403672"}}); err != nil {
		t.Fatal(err)
	}
	merged, err := db.ReconcileLIDs()
	if err != nil {
		t.Fatal(err)
	}
	if merged != 1 {
		t.Errorf("merged = %d, want 1", merged)
	}

	// Verify LID chat is gone.
	lidChat, err := db.GetChat("3917077286968@lid")
	if err != nil {
		t.Fatal(err)
	}
	if lidChat != nil {
		t.Error("LID chat should be deleted after reconciliation")
	}

	// Verify PN chat inherited the LID messages.
	msgs, err := db.ListMessages("558592403672@s.whatsapp.net", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Errorf("PN chat has %d messages, want 2 (original + reassigned from LID)", len(msgs))
	}

	// Verify PN chat got the LID's last_message_at (which was newer).
	pnChat, err := db.GetChat("558592403672@s.whatsapp.net")
	if err != nil {
		t.Fatal(err)
	}
	if pnChat == nil {
		t.Fatal("PN chat should exist after reconciliation")
	}

	// Verify LID contact was merged into PN contact.
	contact, err := db.GetContact("558592403672@s.whatsapp.net")
	if err != nil {
		t.Fatal(err)
	}
	if contact == nil || contact.PushName != "Eric" {
		t.Errorf("PN contact push_name = %v, want Eric (merged from LID contact)", contact)
	}

	// Verify LID contact is gone.
	lidContact, err := db.GetContact("3917077286968@lid")
	if err != nil {
		t.Fatal(err)
	}
	if lidContact != nil {
		t.Error("LID contact should be deleted after reconciliation")
	}
}

// TestListChatsFiltersLIDs verifies that ListChats excludes @lid chats.
func TestListChatsFiltersLIDs(t *testing.T) {
	db := testDB(t)

	if err := db.UpsertChat(&Chat{JID: "user@s.whatsapp.net", LastMessageAt: 1000}); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertChat(&Chat{JID: "12345@lid", LastMessageAt: 2000}); err != nil {
		t.Fatal(err)
	}

	chats, err := db.ListChats(100, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) != 1 {
		t.Errorf("got %d chats, want 1 (@lid should be filtered)", len(chats))
	}
	if len(chats) > 0 && chats[0].JID != "user@s.whatsapp.net" {
		t.Errorf("chat JID = %q, want user@s.whatsapp.net", chats[0].JID)
	}
}

// TestChatAndMessageCounts verifies the count methods.
func TestChatAndMessageCounts(t *testing.T) {
	db := testDB(t)

	count, err := db.ChatCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("chat count = %d, want 0", count)
	}

	if err := db.UpsertChat(&Chat{JID: "c@s"}); err != nil {
		t.Fatal(err)
	}
	count, err = db.ChatCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("chat count = %d, want 1", count)
	}

	if err := db.UpsertMessage(&Message{ChatJID: "c@s", MsgID: "m1", Body: "hi", MessageType: "text", Timestamp: 1000}); err != nil {
		t.Fatal(err)
	}
	mcount, err := db.MessageCount()
	if err != nil {
		t.Fatal(err)
	}
	if mcount != 1 {
		t.Errorf("message count = %d, want 1", mcount)
	}
}
