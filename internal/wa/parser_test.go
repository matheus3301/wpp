package wa

import (
	"testing"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func TestExtractTextBody(t *testing.T) {
	tests := []struct {
		name string
		msg  *waE2E.Message
		want string
	}{
		{"nil message", nil, ""},
		{"conversation", &waE2E.Message{Conversation: proto.String("hello")}, "hello"},
		{"extended text", &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: proto.String("extended")}}, "extended"},
		{"image (no text)", &waE2E.Message{ImageMessage: &waE2E.ImageMessage{}}, ""},
		{"empty conversation", &waE2E.Message{Conversation: proto.String("")}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTextBody(tt.msg)
			if got != tt.want {
				t.Errorf("extractTextBody() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetectMessageType(t *testing.T) {
	tests := []struct {
		name string
		msg  *waE2E.Message
		want string
	}{
		{"nil", nil, "unknown"},
		{"text conversation", &waE2E.Message{Conversation: proto.String("hi")}, "text"},
		{"extended text", &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: proto.String("hi")}}, "text"},
		{"image", &waE2E.Message{ImageMessage: &waE2E.ImageMessage{}}, "image"},
		{"video", &waE2E.Message{VideoMessage: &waE2E.VideoMessage{}}, "video"},
		{"audio", &waE2E.Message{AudioMessage: &waE2E.AudioMessage{}}, "audio"},
		{"document", &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{}}, "document"},
		{"sticker", &waE2E.Message{StickerMessage: &waE2E.StickerMessage{}}, "sticker"},
		{"contact", &waE2E.Message{ContactMessage: &waE2E.ContactMessage{}}, "contact"},
		{"location", &waE2E.Message{LocationMessage: &waE2E.LocationMessage{}}, "location"},
		{"empty message", &waE2E.Message{}, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectMessageType(tt.msg)
			if got != tt.want {
				t.Errorf("detectMessageType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseLiveMessage(t *testing.T) {
	ts := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	evt := &events.Message{
		Info: types.MessageInfo{
			PushName:  "Alice",
			Timestamp: ts,
			MessageSource: types.MessageSource{
				Chat:     types.JID{User: "chat", Server: "s.whatsapp.net"},
				Sender:   types.JID{User: "sender", Server: "s.whatsapp.net"},
				IsFromMe: true,
			},
			ID: "MSG123",
		},
		Message: &waE2E.Message{Conversation: proto.String("hello world")},
	}

	parsed := ParseLiveMessage(evt)

	if parsed.ChatJID != "chat@s.whatsapp.net" {
		t.Errorf("ChatJID = %q, want chat@s.whatsapp.net", parsed.ChatJID)
	}
	if parsed.MsgID != "MSG123" {
		t.Errorf("MsgID = %q, want MSG123", parsed.MsgID)
	}
	if parsed.SenderJID != "sender@s.whatsapp.net" {
		t.Errorf("SenderJID = %q, want sender@s.whatsapp.net", parsed.SenderJID)
	}
	if parsed.SenderName != "Alice" {
		t.Errorf("SenderName = %q, want Alice", parsed.SenderName)
	}
	if parsed.Body != "hello world" {
		t.Errorf("Body = %q, want hello world", parsed.Body)
	}
	if parsed.MessageType != "text" {
		t.Errorf("MessageType = %q, want text", parsed.MessageType)
	}
	if !parsed.FromMe {
		t.Error("FromMe = false, want true")
	}
	if parsed.Timestamp != ts.UnixMilli() {
		t.Errorf("Timestamp = %d, want %d", parsed.Timestamp, ts.UnixMilli())
	}
}

func TestToStoreMessage(t *testing.T) {
	p := &ParsedMessage{
		ChatJID:     "chat@s",
		MsgID:       "m1",
		SenderJID:   "sender@s",
		SenderName:  "Bob",
		Body:        "test",
		MessageType: "text",
		FromMe:      false,
		Timestamp:   42000,
	}

	sm := p.ToStoreMessage()

	if sm.ChatJID != "chat@s" {
		t.Errorf("ChatJID = %q", sm.ChatJID)
	}
	if sm.Status != "received" {
		t.Errorf("Status = %q, want received", sm.Status)
	}
	if sm.FromMe {
		t.Error("FromMe should be false")
	}
}

// TestNormalizeJID verifies that device/agent suffixes are stripped.
// Regression: history sync and live messages produced different JIDs for the
// same contact (e.g. "558592403672:0@s.whatsapp.net" vs "558592403672@s.whatsapp.net"),
// creating duplicate chat entries in the database.
func TestNormalizeJID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"558592403672@s.whatsapp.net", "558592403672@s.whatsapp.net"},
		{"558592403672:0@s.whatsapp.net", "558592403672@s.whatsapp.net"},
		{"558592403672:5@s.whatsapp.net", "558592403672@s.whatsapp.net"},
		{"120363123456@g.us", "120363123456@g.us"},
		{"", ""},
		{"invalid", "invalid"},
		// LID JIDs: NormalizeJID alone cannot resolve these (needs adapter),
		// but it must not crash and should preserve them as-is.
		{"3917077286968@lid", "3917077286968@lid"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeJID(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeJID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestParseLiveMessageStripsDeviceSuffix verifies that live messages from
// device-specific JIDs are normalized to the canonical user JID.
func TestParseLiveMessageStripsDeviceSuffix(t *testing.T) {
	evt := &events.Message{
		Info: types.MessageInfo{
			ID:        "M1",
			Timestamp: time.Now(),
			MessageSource: types.MessageSource{
				Chat:   types.JID{User: "558592403672", Server: "s.whatsapp.net", Device: 1},
				Sender: types.JID{User: "558592403672", Server: "s.whatsapp.net", Device: 3},
			},
		},
		Message: &waE2E.Message{Conversation: proto.String("hi")},
	}

	parsed := ParseLiveMessage(evt)
	if parsed.ChatJID != "558592403672@s.whatsapp.net" {
		t.Errorf("ChatJID = %q, want 558592403672@s.whatsapp.net (device suffix not stripped)", parsed.ChatJID)
	}
	if parsed.SenderJID != "558592403672@s.whatsapp.net" {
		t.Errorf("SenderJID = %q, want 558592403672@s.whatsapp.net (device suffix not stripped)", parsed.SenderJID)
	}
}

func TestParseLiveMessageImageType(t *testing.T) {
	evt := &events.Message{
		Info: types.MessageInfo{
			ID:        "IMG1",
			Timestamp: time.Now(),
			MessageSource: types.MessageSource{
				Chat:   types.JID{User: "c", Server: "s"},
				Sender: types.JID{User: "s", Server: "s"},
			},
		},
		Message: &waE2E.Message{ImageMessage: &waE2E.ImageMessage{}},
	}

	parsed := ParseLiveMessage(evt)
	if parsed.MessageType != "image" {
		t.Errorf("MessageType = %q, want image", parsed.MessageType)
	}
	if parsed.Body != "" {
		t.Errorf("Body = %q, want empty for image", parsed.Body)
	}
}
