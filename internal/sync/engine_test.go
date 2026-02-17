package sync

import (
	"path/filepath"
	"testing"
	"time"

	"context"

	"github.com/matheus3301/wpp/internal/bus"
	"github.com/matheus3301/wpp/internal/store"
	"go.uber.org/zap"
)

func testDB(t *testing.T) *store.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestEngineIngestMessage(t *testing.T) {
	db := testDB(t)
	b := bus.New()
	e := NewEngine(db, b, nil)

	ch, unsub := b.Subscribe("message.", 10)
	defer unsub()

	msg := &store.Message{
		ChatJID: "chat@s", MsgID: "m1", Body: "hello",
		MessageType: "text", Timestamp: 1000,
	}
	if err := e.IngestMessage(msg); err != nil {
		t.Fatal(err)
	}

	// Verify chat was auto-created.
	chat, err := db.GetChat("chat@s")
	if err != nil {
		t.Fatal(err)
	}
	if chat == nil {
		t.Fatal("chat not created")
	}

	// Verify message stored.
	msgs, err := db.ListMessages("chat@s", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 || msgs[0].Body != "hello" {
		t.Errorf("got %d messages, want 1 with body=hello", len(msgs))
	}

	// Verify bus event published.
	select {
	case evt := <-ch:
		if evt.Kind != "message.upserted" {
			t.Errorf("event kind = %q, want message.upserted", evt.Kind)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message.upserted event")
	}
}

func TestEngineIngestMessageIdempotent(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, bus.New(), nil)

	msg := &store.Message{
		ChatJID: "chat@s", MsgID: "m1", Body: "v1",
		MessageType: "text", Timestamp: 1000,
	}
	if err := e.IngestMessage(msg); err != nil {
		t.Fatal(err)
	}
	msg.Body = "v2"
	if err := e.IngestMessage(msg); err != nil {
		t.Fatal(err)
	}

	msgs, err := db.ListMessages("chat@s", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1 (idempotent)", len(msgs))
	}
	if msgs[0].Body != "v2" {
		t.Errorf("body = %q, want v2 (updated)", msgs[0].Body)
	}
}

func TestEngineIngestHistoryBatch(t *testing.T) {
	db := testDB(t)
	b := bus.New()
	e := NewEngine(db, b, nil)

	ch, unsub := b.Subscribe("sync.", 10)
	defer unsub()

	msgs := []*store.Message{
		{ChatJID: "a@s", MsgID: "m1", Body: "one", MessageType: "text", Timestamp: 1000, Status: "received"},
		{ChatJID: "a@s", MsgID: "m2", Body: "two", MessageType: "text", Timestamp: 2000, Status: "received"},
		{ChatJID: "b@s", MsgID: "m3", Body: "three", MessageType: "text", Timestamp: 3000, Status: "received"},
	}

	if err := e.IngestHistoryBatch(msgs); err != nil {
		t.Fatal(err)
	}

	// Verify chats created.
	chats, err := db.ListChats(10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) != 2 {
		t.Errorf("got %d chats, want 2", len(chats))
	}

	// Verify all messages stored.
	msgsA, _ := db.ListMessages("a@s", 0, 10)
	msgsB, _ := db.ListMessages("b@s", 0, 10)
	if len(msgsA) != 2 || len(msgsB) != 1 {
		t.Errorf("got %d+%d messages, want 2+1", len(msgsA), len(msgsB))
	}

	// Verify bus event.
	select {
	case evt := <-ch:
		if evt.Kind != "sync.history_batch" {
			t.Errorf("event kind = %q, want sync.history_batch", evt.Kind)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for sync.history_batch event")
	}
}

func TestEngineHistoryBatchIdempotent(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, bus.New(), nil)

	msgs := []*store.Message{
		{ChatJID: "a@s", MsgID: "m1", Body: "hello", MessageType: "text", Timestamp: 1000, Status: "received"},
	}

	// Ingest twice.
	if err := e.IngestHistoryBatch(msgs); err != nil {
		t.Fatal(err)
	}
	if err := e.IngestHistoryBatch(msgs); err != nil {
		t.Fatal(err)
	}

	stored, _ := db.ListMessages("a@s", 0, 10)
	if len(stored) != 1 {
		t.Errorf("got %d messages, want 1 (idempotent batch)", len(stored))
	}
}

// TestEngineBusSubscription verifies the engine processes events from the bus.
// This is the core of the wa→bus→sync decoupling.
func TestEngineBusSubscription(t *testing.T) {
	db := testDB(t)
	b := bus.New()
	logger, _ := zap.NewDevelopment()
	e := NewEngine(db, b, logger)

	ctx := context.Background()
	e.Start(ctx)
	defer e.Stop()

	// Publish a wa.message event (simulating what wa.EventHandler does).
	b.Publish(bus.Event{
		Kind:      "wa.message",
		Timestamp: time.Now(),
		Payload: &store.Message{
			ChatJID: "bus-test@s", MsgID: "bm1", Body: "from bus",
			MessageType: "text", Timestamp: 5000, Status: "received",
		},
	})

	// Give the engine time to process.
	time.Sleep(100 * time.Millisecond)

	msgs, err := db.ListMessages("bus-test@s", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1 (bus subscription)", len(msgs))
	}
	if msgs[0].Body != "from bus" {
		t.Errorf("body = %q, want 'from bus'", msgs[0].Body)
	}

	// Publish a wa.history_batch event.
	b.Publish(bus.Event{
		Kind:      "wa.history_batch",
		Timestamp: time.Now(),
		Payload: []*store.Message{
			{ChatJID: "batch@s", MsgID: "hm1", Body: "history", MessageType: "text", Timestamp: 6000, Status: "received"},
			{ChatJID: "batch@s", MsgID: "hm2", Body: "history2", MessageType: "text", Timestamp: 7000, Status: "received"},
		},
	})

	time.Sleep(100 * time.Millisecond)

	msgs, err = db.ListMessages("batch@s", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Errorf("got %d messages, want 2 (history batch via bus)", len(msgs))
	}
}
