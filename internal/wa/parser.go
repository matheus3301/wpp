package wa

import (
	"github.com/matheus3301/wpp/internal/store"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// ParsedMessage is a normalized message ready for ingestion.
type ParsedMessage struct {
	ChatJID     string
	MsgID       string
	SenderJID   string
	SenderName  string
	Body        string
	MessageType string
	FromMe      bool
	Timestamp   int64
}

// ParseLiveMessage normalizes a live whatsmeow message event.
func ParseLiveMessage(evt *events.Message) *ParsedMessage {
	body := extractTextBody(evt.Message)
	msgType := detectMessageType(evt.Message)

	return &ParsedMessage{
		ChatJID:     evt.Info.Chat.String(),
		MsgID:       evt.Info.ID,
		SenderJID:   evt.Info.Sender.String(),
		SenderName:  evt.Info.PushName,
		Body:        body,
		MessageType: msgType,
		FromMe:      evt.Info.IsFromMe,
		Timestamp:   evt.Info.Timestamp.UnixMilli(),
	}
}

// ParseHistoryMessage normalizes a history sync message.
func ParseHistoryMessage(msg *waE2E.Message, info types.MessageInfo) *ParsedMessage {
	body := extractTextBody(msg)
	msgType := detectMessageType(msg)

	return &ParsedMessage{
		ChatJID:     info.Chat.String(),
		MsgID:       info.ID,
		SenderJID:   info.Sender.String(),
		SenderName:  info.PushName,
		Body:        body,
		MessageType: msgType,
		FromMe:      info.IsFromMe,
		Timestamp:   info.Timestamp.UnixMilli(),
	}
}

// ToStoreMessage converts a ParsedMessage to a store.Message.
func (p *ParsedMessage) ToStoreMessage() *store.Message {
	return &store.Message{
		ChatJID:     p.ChatJID,
		MsgID:       p.MsgID,
		SenderJID:   p.SenderJID,
		SenderName:  p.SenderName,
		Body:        p.Body,
		MessageType: p.MessageType,
		FromMe:      p.FromMe,
		Status:      "received",
		Timestamp:   p.Timestamp,
	}
}

func extractTextBody(msg *waE2E.Message) string {
	if msg == nil {
		return ""
	}
	if c := msg.GetConversation(); c != "" {
		return c
	}
	if ext := msg.GetExtendedTextMessage(); ext != nil {
		return ext.GetText()
	}
	return ""
}

func detectMessageType(msg *waE2E.Message) string {
	if msg == nil {
		return "unknown"
	}
	switch {
	case msg.GetConversation() != "" || msg.GetExtendedTextMessage() != nil:
		return "text"
	case msg.GetImageMessage() != nil:
		return "image"
	case msg.GetVideoMessage() != nil:
		return "video"
	case msg.GetAudioMessage() != nil:
		return "audio"
	case msg.GetDocumentMessage() != nil:
		return "document"
	case msg.GetStickerMessage() != nil:
		return "sticker"
	case msg.GetContactMessage() != nil:
		return "contact"
	case msg.GetLocationMessage() != nil:
		return "location"
	default:
		return "unknown"
	}
}
