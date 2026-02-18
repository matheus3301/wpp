package wa

import (
	"context"
	"testing"
	"time"

	"github.com/matheus3301/wpp/internal/bus"
	"github.com/matheus3301/wpp/internal/status"
	"github.com/matheus3301/wpp/internal/store"
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
	h := NewEventHandler(b, m, nil, logger)

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
	h := NewEventHandler(b, m, nil, logger)

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
	h := NewEventHandler(b, m, nil, logger)

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
	h := NewEventHandler(b, m, nil, logger)

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
	h := NewEventHandler(b, m, nil, logger)

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
	h := NewEventHandler(b, m, nil, logger)

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
	h := NewEventHandler(b, m, nil, logger)

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
	h := NewEventHandler(b, m, nil, logger)

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
	h := NewEventHandler(b, m, nil, logger)

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

// --- LID resolution regression tests ---
// Regression: WhatsApp uses LID (Linked Identity) JIDs like "3917077286968@lid"
// alongside phone number JIDs like "558592403672@s.whatsapp.net" for the same
// user. Without LID resolution, these appear as duplicate chats in the TUI.

// TestResolveJIDWithNilAdapter verifies that resolveJID works with nil adapter
// (fallback to NormalizeJID only — strips device suffix but cannot resolve LIDs).
func TestResolveJIDWithNilAdapter(t *testing.T) {
	b := bus.New()
	m := status.NewMachine(b)
	h := NewEventHandler(b, m, nil, zap.NewNop())

	tests := []struct {
		input string
		want  string
	}{
		{"558592403672@s.whatsapp.net", "558592403672@s.whatsapp.net"},
		{"558592403672:0@s.whatsapp.net", "558592403672@s.whatsapp.net"},
		// LID cannot be resolved without adapter, stays as-is.
		{"3917077286968@lid", "3917077286968@lid"},
	}

	for _, tt := range tests {
		got := h.resolveJID(tt.input)
		if got != tt.want {
			t.Errorf("resolveJID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestResolveLIDNonLIDPassthrough verifies that ResolveLID passes through
// non-LID JIDs unchanged.
func TestResolveLIDNonLIDPassthrough(t *testing.T) {
	a := &Adapter{}
	regular := types.JID{User: "558592403672", Server: "s.whatsapp.net"}
	got := a.ResolveLID(context.Background(), regular)
	if got != regular {
		t.Errorf("ResolveLID(regular) = %v, want %v (should pass through)", got, regular)
	}

	// Group JIDs should also pass through.
	group := types.JID{User: "120363123456", Server: "g.us"}
	got = a.ResolveLID(context.Background(), group)
	if got != group {
		t.Errorf("ResolveLID(group) = %v, want %v (should pass through)", got, group)
	}
}

// TestResolveLIDDetectsHiddenUserServer verifies that ResolveLID recognizes
// @lid JIDs (HiddenUserServer). Without a real LID store, it returns the
// original JID, but the detection path is exercised.
func TestResolveLIDDetectsHiddenUserServer(t *testing.T) {
	a := &Adapter{}
	// With nil client.Store.LIDs, ResolveLID should return the original JID
	// but NOT skip the LID check (regression: was comparing against HostedLIDServer
	// instead of HiddenUserServer, so @lid JIDs were never resolved).
	lid := types.JID{User: "3917077286968", Server: types.HiddenUserServer}
	got := a.ResolveLID(context.Background(), lid)
	// Without LIDs store, returns original.
	if got != lid {
		t.Errorf("ResolveLID(lid, nil store) = %v, want %v", got, lid)
	}
}

// TestLiveMessageWithDeviceSuffixNormalized verifies that live messages from
// device-specific JIDs produce normalized chat/sender JIDs in bus events.
// Regression: device JIDs like "user:0@s.whatsapp.net" created separate chats.
func TestLiveMessageWithDeviceSuffixNormalized(t *testing.T) {
	b := bus.New()
	m := status.NewMachine(b)
	h := NewEventHandler(b, m, nil, zap.NewNop())

	walkTo(t, m, status.Connecting, status.Syncing)

	ch, unsub := b.Subscribe("wa.message", 10)
	defer unsub()

	h.Handle(&events.Message{
		Info: types.MessageInfo{
			ID:        "m1",
			Timestamp: time.Now(),
			MessageSource: types.MessageSource{
				Chat:   types.JID{User: "558592403672", Server: "s.whatsapp.net", Device: 1},
				Sender: types.JID{User: "558592403672", Server: "s.whatsapp.net", Device: 3},
			},
		},
		Message: &waE2E.Message{Conversation: proto.String("hello")},
	})

	select {
	case evt := <-ch:
		msg, ok := evt.Payload.(*store.Message)
		if !ok {
			t.Fatal("payload is not *store.Message")
		}
		if msg.ChatJID != "558592403672@s.whatsapp.net" {
			t.Errorf("ChatJID = %q, want 558592403672@s.whatsapp.net (device suffix not stripped)", msg.ChatJID)
		}
		if msg.SenderJID != "558592403672@s.whatsapp.net" {
			t.Errorf("SenderJID = %q, want 558592403672@s.whatsapp.net (device suffix not stripped)", msg.SenderJID)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for wa.message event")
	}
}

// TestHistorySyncWithLIDConversation verifies that history sync conversations
// using LID JIDs are passed through resolveJID. Without an adapter, LIDs stay
// as-is, but the plumbing is exercised. With a real adapter, they'd be resolved.
func TestHistorySyncWithLIDConversation(t *testing.T) {
	b := bus.New()
	m := status.NewMachine(b)
	h := NewEventHandler(b, m, nil, zap.NewNop())

	walkTo(t, m, status.Connecting, status.Syncing)

	ch, unsub := b.Subscribe("wa.", 10)
	defer unsub()

	msgTS := uint64(time.Now().Unix())
	h.Handle(&events.HistorySync{
		Data: &waHistorySync.HistorySync{
			Conversations: []*waHistorySync.Conversation{
				{
					// LID JID as conversation ID — this is the real-world scenario.
					ID:   proto.String("3917077286968@lid"),
					Name: proto.String("Eric"),
					Messages: []*waHistorySync.HistorySyncMsg{
						{
							Message: &waWeb.WebMessageInfo{
								Key: &waCommon.MessageKey{
									ID:          proto.String("hm1"),
									FromMe:      proto.Bool(false),
									RemoteJID:   proto.String("3917077286968@lid"),
									Participant: proto.String("3917077286968@lid"),
								},
								MessageTimestamp: &msgTS,
								Message:          &waE2E.Message{Conversation: proto.String("test msg")},
								PushName:         proto.String("Eric"),
							},
						},
					},
				},
			},
		},
	})

	// Collect all emitted events.
	var batchEvt, contactEvt bus.Event
	timeout := time.After(time.Second)
loop:
	for i := 0; i < 2; i++ {
		select {
		case evt := <-ch:
			switch evt.Kind {
			case "wa.history_batch":
				batchEvt = evt
			case "wa.contact_batch":
				contactEvt = evt
			}
		case <-timeout:
			break loop
		}
	}

	// Verify the history batch was emitted with the LID JID
	// (it goes through resolveJID, which without adapter, keeps it as-is).
	if batchEvt.Kind != "wa.history_batch" {
		t.Fatal("did not receive wa.history_batch event")
	}
	msgs, ok := batchEvt.Payload.([]*store.Message)
	if !ok || len(msgs) == 0 {
		t.Fatal("history batch has no messages")
	}
	// Without adapter, LID stays as-is (NormalizeJID can't resolve it).
	if msgs[0].ChatJID != "3917077286968@lid" {
		t.Errorf("ChatJID = %q, want 3917077286968@lid (unresolved without adapter)", msgs[0].ChatJID)
	}

	// Verify contact batch was emitted with conversation name.
	if contactEvt.Kind != "wa.contact_batch" {
		t.Fatal("did not receive wa.contact_batch event")
	}
	contacts, ok := contactEvt.Payload.([]*store.Contact)
	if !ok || len(contacts) == 0 {
		t.Fatal("contact batch is empty")
	}
	// Should have the conversation name contact.
	foundName := false
	for _, c := range contacts {
		if c.Name == "Eric" {
			foundName = true
		}
	}
	if !foundName {
		t.Error("contact batch should contain a contact with Name=Eric from conversation metadata")
	}
}

// TestHistorySyncDeviceSuffixStripped verifies that history sync conversations
// with device-suffix JIDs are normalized to plain JIDs.
func TestHistorySyncDeviceSuffixStripped(t *testing.T) {
	b := bus.New()
	m := status.NewMachine(b)
	h := NewEventHandler(b, m, nil, zap.NewNop())

	walkTo(t, m, status.Connecting, status.Syncing)

	ch, unsub := b.Subscribe("wa.history_batch", 10)
	defer unsub()

	msgTS := uint64(time.Now().Unix())
	h.Handle(&events.HistorySync{
		Data: &waHistorySync.HistorySync{
			Conversations: []*waHistorySync.Conversation{
				{
					// Device-suffix JID.
					ID: proto.String("558592403672:0@s.whatsapp.net"),
					Messages: []*waHistorySync.HistorySyncMsg{
						{
							Message: &waWeb.WebMessageInfo{
								Key: &waCommon.MessageKey{
									ID:          proto.String("hm1"),
									FromMe:      proto.Bool(false),
									RemoteJID:   proto.String("558592403672:0@s.whatsapp.net"),
									Participant: proto.String("558592403672:2@s.whatsapp.net"),
								},
								MessageTimestamp: &msgTS,
								Message:          &waE2E.Message{Conversation: proto.String("hello")},
							},
						},
					},
				},
			},
		},
	})

	select {
	case evt := <-ch:
		msgs, ok := evt.Payload.([]*store.Message)
		if !ok || len(msgs) == 0 {
			t.Fatal("history batch has no messages")
		}
		if msgs[0].ChatJID != "558592403672@s.whatsapp.net" {
			t.Errorf("ChatJID = %q, want 558592403672@s.whatsapp.net (device suffix not stripped)", msgs[0].ChatJID)
		}
		if msgs[0].SenderJID != "558592403672@s.whatsapp.net" {
			t.Errorf("SenderJID = %q, want 558592403672@s.whatsapp.net (device suffix not stripped)", msgs[0].SenderJID)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for wa.history_batch event")
	}
}

// TestPushNameContactJIDNormalized verifies that PushName events produce
// contact entries with normalized JIDs (no device suffix).
func TestPushNameContactJIDNormalized(t *testing.T) {
	b := bus.New()
	m := status.NewMachine(b)
	h := NewEventHandler(b, m, nil, zap.NewNop())

	ch, unsub := b.Subscribe("wa.contact", 10)
	defer unsub()

	h.Handle(&events.PushName{
		JID:         types.JID{User: "558592403672", Server: "s.whatsapp.net", Device: 5},
		NewPushName: "Eric",
	})

	select {
	case evt := <-ch:
		contact, ok := evt.Payload.(*store.Contact)
		if !ok {
			t.Fatal("payload is not *store.Contact")
		}
		if contact.JID != "558592403672@s.whatsapp.net" {
			t.Errorf("JID = %q, want 558592403672@s.whatsapp.net (device suffix not stripped)", contact.JID)
		}
		if contact.PushName != "Eric" {
			t.Errorf("PushName = %q, want Eric", contact.PushName)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for wa.contact event")
	}
}
