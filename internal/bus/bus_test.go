package bus

import (
	"testing"
	"time"
)

func TestPublishSubscribe(t *testing.T) {
	b := New()
	ch, unsub := b.Subscribe("session.", 10)
	defer unsub()

	b.Publish(Event{Kind: "session.status_changed", Timestamp: time.Now(), Payload: "test"})

	select {
	case evt := <-ch:
		if evt.Kind != "session.status_changed" {
			t.Errorf("got kind %q, want session.status_changed", evt.Kind)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestNamespaceFiltering(t *testing.T) {
	b := New()
	ch, unsub := b.Subscribe("sync.", 10)
	defer unsub()

	b.Publish(Event{Kind: "session.status_changed"})
	b.Publish(Event{Kind: "sync.connected"})

	select {
	case evt := <-ch:
		if evt.Kind != "sync.connected" {
			t.Errorf("got kind %q, want sync.connected", evt.Kind)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	// Ensure session event was not delivered.
	select {
	case evt := <-ch:
		t.Errorf("unexpected event: %v", evt)
	case <-time.After(50 * time.Millisecond):
		// Expected: no more events.
	}
}

func TestUnsubscribe(t *testing.T) {
	b := New()
	ch, unsub := b.Subscribe("session.", 10)
	unsub()

	b.Publish(Event{Kind: "session.status_changed"})

	select {
	case evt := <-ch:
		t.Errorf("received event after unsubscribe: %v", evt)
	case <-time.After(50 * time.Millisecond):
		// Expected.
	}
}

func TestDropOnFullBuffer(t *testing.T) {
	b := New()
	ch, unsub := b.Subscribe("test.", 1)
	defer unsub()

	// Fill buffer.
	b.Publish(Event{Kind: "test.one"})
	// This should be dropped (non-blocking).
	b.Publish(Event{Kind: "test.two"})

	evt := <-ch
	if evt.Kind != "test.one" {
		t.Errorf("got %q, want test.one", evt.Kind)
	}
}
