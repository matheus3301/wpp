package store

// Chat represents a synced chat.
type Chat struct {
	JID                string
	Name               string
	IsGroup            bool
	UnreadCount        int
	LastMessageAt      int64
	LastMessagePreview string
}

// Contact represents a synced contact.
type Contact struct {
	JID      string
	Name     string
	PushName string
}

// Message represents a synced message.
type Message struct {
	ID          int64
	ChatJID     string
	MsgID       string
	SenderJID   string
	SenderName  string
	Body        string
	MessageType string
	FromMe      bool
	Status      string
	Timestamp   int64
}

// OutboxEntry represents a pending outgoing message.
type OutboxEntry struct {
	ID           int64
	ClientMsgID  string
	ChatJID      string
	Body         string
	Status       string // queued, sending, sent, failed
	ErrorMessage string
	ServerMsgID  string
}

// SearchResult holds a message with a search snippet.
type SearchResult struct {
	Message Message
	Snippet string
}
