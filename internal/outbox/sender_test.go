package outbox

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/matheus3301/wpp/internal/bus"
	"github.com/matheus3301/wpp/internal/store"
	"go.uber.org/zap"
)

// mockSender records calls and returns configurable results.
type mockSender struct {
	calls []sendCall
	err   error
	delay time.Duration // artificial delay to observe intermediate states
}

type sendCall struct {
	JID  string
	Text string
}

func (m *mockSender) SendText(_ context.Context, jid string, text string) (string, error) {
	m.calls = append(m.calls, sendCall{JID: jid, Text: text})
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	if m.err != nil {
		return "", m.err
	}
	return "server-" + jid, nil
}

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

func TestSenderProcessesPendingMessages(t *testing.T) {
	db := testDB(t)
	b := bus.New()
	mock := &mockSender{}
	logger, _ := zap.NewDevelopment()
	s := NewSender(db, mock, b, nil, logger)

	// Subscribe to ack events.
	ch, unsub := b.Subscribe("message.send_ack", 10)
	defer unsub()

	// Queue a message.
	if err := db.UpsertChat(&store.Chat{JID: "chat@s"}); err != nil {
		t.Fatal(err)
	}
	if err := db.QueueOutbox("c1", "chat@s", "hello"); err != nil {
		t.Fatal(err)
	}

	// Start sender and wait for it to process.
	s.Start(context.Background())
	defer s.Stop()

	time.Sleep(time.Second)

	// Verify the mock was called.
	if len(mock.calls) != 1 {
		t.Fatalf("got %d send calls, want 1", len(mock.calls))
	}
	if mock.calls[0].JID != "chat@s" || mock.calls[0].Text != "hello" {
		t.Errorf("call = %+v, want {chat@s, hello}", mock.calls[0])
	}

	// Verify outbox is drained (no more pending).
	pending, err := db.PendingOutbox()
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Errorf("got %d pending, want 0 after send", len(pending))
	}

	// Verify ack event published.
	select {
	case evt := <-ch:
		if evt.Kind != "message.send_ack" {
			t.Errorf("event kind = %q, want message.send_ack", evt.Kind)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for send_ack event")
	}
}

func TestSenderHandlesFailure(t *testing.T) {
	db := testDB(t)
	b := bus.New()
	mock := &mockSender{err: fmt.Errorf("network error")}
	logger, _ := zap.NewDevelopment()
	s := NewSender(db, mock, b, nil, logger)

	// Subscribe to failure events.
	ch, unsub := b.Subscribe("message.send_failed", 10)
	defer unsub()

	if err := db.UpsertChat(&store.Chat{JID: "chat@s"}); err != nil {
		t.Fatal(err)
	}
	if err := db.QueueOutbox("c1", "chat@s", "hello"); err != nil {
		t.Fatal(err)
	}

	s.Start(context.Background())
	defer s.Stop()

	time.Sleep(time.Second)

	// Verify failure event published.
	select {
	case evt := <-ch:
		if evt.Kind != "message.send_failed" {
			t.Errorf("event kind = %q, want message.send_failed", evt.Kind)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for send_failed event")
	}

	// Verify outbox entry is no longer pending (marked failed).
	pending, err := db.PendingOutbox()
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Errorf("got %d pending, want 0 (should be marked failed)", len(pending))
	}
}

// TestSenderOptimisticInsert verifies that the outbox inserts a message with
// status "sending" into the messages table before the actual send completes,
// then updates to "sent" after success.
// Regression: sent messages didn't appear in the TUI because nothing wrote
// to the messages table until whatsmeow echoed the message back.
func TestSenderOptimisticInsert(t *testing.T) {
	db := testDB(t)
	b := bus.New()
	mock := &mockSender{delay: 500 * time.Millisecond}
	logger, _ := zap.NewDevelopment()
	s := NewSender(db, mock, b, nil, logger)

	if err := db.UpsertChat(&store.Chat{JID: "chat@s"}); err != nil {
		t.Fatal(err)
	}
	if err := db.QueueOutbox("c1", "chat@s", "optimistic"); err != nil {
		t.Fatal(err)
	}

	// Subscribe to upserted events to know when the optimistic insert happened.
	ch, unsub := b.Subscribe("message.upserted", 10)
	defer unsub()

	s.Start(context.Background())
	defer s.Stop()

	// Wait for the optimistic insert (before the mock's 500ms delay finishes).
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for optimistic message.upserted event")
	}

	// Message should exist with status "sending" while mock is still sleeping.
	msgs, err := db.ListMessages("chat@s", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1 (optimistic insert)", len(msgs))
	}
	if msgs[0].Status != "sending" {
		t.Errorf("status = %q, want 'sending' (optimistic)", msgs[0].Status)
	}
	if msgs[0].Body != "optimistic" {
		t.Errorf("body = %q, want 'optimistic'", msgs[0].Body)
	}
	if !msgs[0].FromMe {
		t.Error("from_me = false, want true")
	}

	// Wait for send to complete.
	time.Sleep(time.Second)

	// Message should now have status "sent".
	msgs, err = db.ListMessages("chat@s", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	if msgs[0].Status != "sent" {
		t.Errorf("final status = %q, want 'sent'", msgs[0].Status)
	}
}

// TestSenderOptimisticInsertOnFailure verifies that a failed send updates
// the optimistic message to "failed" status.
func TestSenderOptimisticInsertOnFailure(t *testing.T) {
	db := testDB(t)
	b := bus.New()
	mock := &mockSender{err: fmt.Errorf("timeout"), delay: 200 * time.Millisecond}
	logger, _ := zap.NewDevelopment()
	s := NewSender(db, mock, b, nil, logger)

	if err := db.UpsertChat(&store.Chat{JID: "chat@s"}); err != nil {
		t.Fatal(err)
	}
	if err := db.QueueOutbox("c1", "chat@s", "will-fail"); err != nil {
		t.Fatal(err)
	}

	s.Start(context.Background())
	defer s.Stop()

	time.Sleep(time.Second)

	msgs, err := db.ListMessages("chat@s", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	if msgs[0].Status != "failed" {
		t.Errorf("status = %q, want 'failed'", msgs[0].Status)
	}
}
