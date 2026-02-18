package wa

import (
	"testing"
	"time"

	"github.com/matheus3301/wpp/internal/bus"
	"github.com/matheus3301/wpp/internal/status"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/proto/waHistorySync"
	"go.mau.fi/whatsmeow/proto/waWeb"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

// walkTo transitions the machine through the given states sequentially.
func walkTo(t *testing.T, m *status.Machine, states ...status.State) {
	t.Helper()
	for _, s := range states {
		if err := m.Transition(s); err != nil {
			t.Fatalf("transition to %s failed: %v", s, err)
		}
	}
}

func TestHandleConnectedFromAuthRequired(t *testing.T) {
	b := bus.New()
	m := status.NewMachine(b)
	logger := zap.NewNop()
	h := NewEventHandler(b, m, logger)

	walkTo(t, m, status.AuthRequired)

	ch, unsub := b.Subscribe("sync.", 10)
	defer unsub()

	h.Handle(&events.Connected{})

	if m.Current() != status.Syncing {
		t.Errorf("state = %s, want SYNCING", m.Current())
	}

	select {
	case evt := <-ch:
		if evt.Kind != "sync.connected" {
			t.Errorf("event kind = %q, want sync.connected", evt.Kind)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for sync.connected event")
	}
}

func TestHandleConnectedFromReconnecting(t *testing.T) {
	b := bus.New()
	m := status.NewMachine(b)
	logger := zap.NewNop()
	h := NewEventHandler(b, m, logger)

	walkTo(t, m, status.Connecting, status.Syncing, status.Reconnecting)

	h.Handle(&events.Connected{})

	if m.Current() != status.Syncing {
		t.Errorf("state = %s, want SYNCING (reconnect path)", m.Current())
	}
}

func TestHandleConnectedFromConnecting(t *testing.T) {
	b := bus.New()
	m := status.NewMachine(b)
	logger := zap.NewNop()
	h := NewEventHandler(b, m, logger)

	walkTo(t, m, status.Connecting)

	h.Handle(&events.Connected{})

	if m.Current() != status.Syncing {
		t.Errorf("state = %s, want SYNCING", m.Current())
	}
}

func TestHandleDisconnected(t *testing.T) {
	b := bus.New()
	m := status.NewMachine(b)
	logger := zap.NewNop()
	h := NewEventHandler(b, m, logger)

	walkTo(t, m, status.Connecting, status.Syncing, status.Ready)

	ch, unsub := b.Subscribe("sync.", 10)
	defer unsub()

	h.Handle(&events.Disconnected{})

	if m.Current() != status.Reconnecting {
		t.Errorf("state = %s, want RECONNECTING", m.Current())
	}

	select {
	case evt := <-ch:
		if evt.Kind != "sync.disconnected" {
			t.Errorf("event kind = %q, want sync.disconnected", evt.Kind)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for sync.disconnected event")
	}
}

func TestHandleMessageTransitionsToReady(t *testing.T) {
	b := bus.New()
	m := status.NewMachine(b)
	logger := zap.NewNop()
	h := NewEventHandler(b, m, logger)

	walkTo(t, m, status.Connecting, status.Syncing)

	ch, unsub := b.Subscribe("wa.", 10)
	defer unsub()

	h.Handle(&events.Message{
		Info: types.MessageInfo{
			ID:        "test1",
			Timestamp: time.Now(),
			MessageSource: types.MessageSource{
				Chat:   types.JID{User: "c", Server: "s"},
				Sender: types.JID{User: "s", Server: "s"},
			},
		},
		Message: &waE2E.Message{Conversation: proto.String("hello")},
	})

	if m.Current() != status.Ready {
		t.Errorf("state = %s, want READY (first message after sync)", m.Current())
	}

	select {
	case evt := <-ch:
		if evt.Kind != "wa.message" {
			t.Errorf("event kind = %q, want wa.message", evt.Kind)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for wa.message event")
	}
}

func TestHandleMessageWhileReady(t *testing.T) {
	b := bus.New()
	m := status.NewMachine(b)
	logger := zap.NewNop()
	h := NewEventHandler(b, m, logger)

	walkTo(t, m, status.Connecting, status.Syncing, status.Ready)

	h.Handle(&events.Message{
		Info: types.MessageInfo{
			ID:        "test2",
			Timestamp: time.Now(),
			MessageSource: types.MessageSource{
				Chat:   types.JID{User: "c", Server: "s"},
				Sender: types.JID{User: "s", Server: "s"},
			},
		},
		Message: &waE2E.Message{Conversation: proto.String("hello again")},
	})

	if m.Current() != status.Ready {
		t.Errorf("state = %s, want READY (should stay ready)", m.Current())
	}
}

func TestHandleLoggedOut(t *testing.T) {
	b := bus.New()
	m := status.NewMachine(b)
	logger := zap.NewNop()
	h := NewEventHandler(b, m, logger)

	walkTo(t, m, status.Connecting, status.Syncing, status.Ready)

	ch, unsub := b.Subscribe("session.", 10)
	defer unsub()

	h.Handle(&events.LoggedOut{})

	if m.Current() != status.AuthRequired {
		t.Errorf("state = %s, want AUTH_REQUIRED", m.Current())
	}

	// Drain status_changed events to find logged_out.
	found := false
	timeout := time.After(time.Second)
drain:
	for {
		select {
		case evt := <-ch:
			if evt.Kind == "session.logged_out" {
				found = true
				break drain
			}
		case <-timeout:
			break drain
		}
	}
	if !found {
		t.Error("did not receive session.logged_out event")
	}
}

func TestHandleHistorySync(t *testing.T) {
	b := bus.New()
	m := status.NewMachine(b)
	logger := zap.NewNop()
	h := NewEventHandler(b, m, logger)

	walkTo(t, m, status.Connecting, status.Syncing)

	ch, unsub := b.Subscribe("wa.", 10)
	defer unsub()

	msgTS := uint64(time.Now().Unix())
	h.Handle(&events.HistorySync{
		Data: &waHistorySync.HistorySync{
			Conversations: []*waHistorySync.Conversation{
				{
					ID: proto.String("chat@g.us"),
					Messages: []*waHistorySync.HistorySyncMsg{
						{
							Message: &waWeb.WebMessageInfo{
								Key: &waCommon.MessageKey{
									ID:        proto.String("hm1"),
									FromMe:    proto.Bool(false),
									RemoteJID: proto.String("chat@g.us"),
								},
								MessageTimestamp: &msgTS,
								Message:          &waE2E.Message{Conversation: proto.String("history msg")},
							},
						},
					},
				},
			},
		},
	})

	select {
	case evt := <-ch:
		if evt.Kind != "wa.history_batch" {
			t.Errorf("event kind = %q, want wa.history_batch", evt.Kind)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for wa.history_batch event")
	}
}

func TestHandleHistorySyncNilData(t *testing.T) {
	b := bus.New()
	m := status.NewMachine(b)
	logger := zap.NewNop()
	h := NewEventHandler(b, m, logger)

	walkTo(t, m, status.Connecting, status.Syncing)

	ch, unsub := b.Subscribe("wa.", 10)
	defer unsub()

	// Should not panic on nil data.
	h.Handle(&events.HistorySync{Data: nil})

	select {
	case evt := <-ch:
		t.Errorf("unexpected event: %v", evt)
	case <-time.After(50 * time.Millisecond):
		// Expected: no events.
	}
}
