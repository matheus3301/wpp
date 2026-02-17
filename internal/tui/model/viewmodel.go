package model

import (
	"context"
	"io"
	"log"
	"sync"
	"time"

	wppv1 "github.com/matheus3301/wpp/gen/wpp/v1"
	"github.com/matheus3301/wpp/internal/tui/client"
)

// ViewModel caches state from gRPC streams and signals UI refreshes.
type ViewModel struct {
	mu sync.RWMutex

	client        *client.Client
	SessionStatus *wppv1.GetSessionStatusResponse
	SyncStatus    *wppv1.GetSyncStatusResponse
	Chats         []*wppv1.Chat
	Messages      []*wppv1.Message
	ActiveChatJID string
	Flash         Flash

	refreshCh chan struct{}
}

// NewViewModel creates a new view model connected to the daemon client.
func NewViewModel(c *client.Client) *ViewModel {
	return &ViewModel{
		client:    c,
		refreshCh: make(chan struct{}, 1),
	}
}

// RefreshCh returns the channel that signals UI refresh.
func (vm *ViewModel) RefreshCh() <-chan struct{} {
	return vm.refreshCh
}

// SignalRefresh sends a non-blocking signal on the refresh channel.
func (vm *ViewModel) SignalRefresh() {
	select {
	case vm.refreshCh <- struct{}{}:
	default:
	}
}

// LoadSessionStatus fetches current session status.
func (vm *ViewModel) LoadSessionStatus(ctx context.Context) error {
	resp, err := vm.client.Session.GetSessionStatus(ctx, &wppv1.GetSessionStatusRequest{})
	if err != nil {
		return err
	}
	vm.mu.Lock()
	vm.SessionStatus = resp
	vm.mu.Unlock()
	vm.SignalRefresh()
	return nil
}

// LoadSyncStatus fetches current sync status.
func (vm *ViewModel) LoadSyncStatus(ctx context.Context) error {
	resp, err := vm.client.Sync.GetSyncStatus(ctx, &wppv1.GetSyncStatusRequest{})
	if err != nil {
		return err
	}
	vm.mu.Lock()
	vm.SyncStatus = resp
	vm.mu.Unlock()
	vm.SignalRefresh()
	return nil
}

// LoadChats fetches the chat list.
func (vm *ViewModel) LoadChats(ctx context.Context) error {
	resp, err := vm.client.Chat.ListChats(ctx, &wppv1.ListChatsRequest{
		Pagination: &wppv1.Pagination{Limit: 100},
	})
	if err != nil {
		return err
	}
	vm.mu.Lock()
	vm.Chats = resp.Chats
	vm.mu.Unlock()
	vm.SignalRefresh()
	return nil
}

// LoadMessages fetches messages for the active chat.
func (vm *ViewModel) LoadMessages(ctx context.Context, chatJID string) error {
	resp, err := vm.client.Message.ListMessages(ctx, &wppv1.ListMessagesRequest{
		ChatJid:    chatJID,
		Pagination: &wppv1.Pagination{Limit: 100},
	})
	if err != nil {
		return err
	}
	vm.mu.Lock()
	vm.ActiveChatJID = chatJID
	vm.Messages = resp.Messages
	vm.mu.Unlock()
	vm.SignalRefresh()
	return nil
}

// SearchMessages performs a search query.
func (vm *ViewModel) SearchMessages(ctx context.Context, query string) ([]*wppv1.SearchResult, error) {
	resp, err := vm.client.Message.SearchMessages(ctx, &wppv1.SearchMessagesRequest{
		Query:      query,
		Pagination: &wppv1.Pagination{Limit: 50},
	})
	if err != nil {
		return nil, err
	}
	return resp.Results, nil
}

// SendText sends a text message.
func (vm *ViewModel) SendText(ctx context.Context, chatJID, text, clientMsgID string) error {
	resp, err := vm.client.Message.SendText(ctx, &wppv1.SendTextRequest{
		ClientMsgId: clientMsgID,
		ChatJid:     chatJID,
		Text:        text,
	})
	if err != nil {
		return err
	}
	if resp.Accepted {
		vm.Flash.Set("Message sent", 3*time.Second)
	}
	vm.SignalRefresh()
	return nil
}

// GetChats returns a snapshot of the current chat list.
func (vm *ViewModel) GetChats() []*wppv1.Chat {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	return vm.Chats
}

// GetMessages returns a snapshot of the current messages.
func (vm *ViewModel) GetMessages() []*wppv1.Message {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	return vm.Messages
}

// GetSessionStatus returns a snapshot of session status.
func (vm *ViewModel) GetSessionStatus() *wppv1.GetSessionStatusResponse {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	return vm.SessionStatus
}

// GetActiveChatJID returns the JID of the currently active chat.
func (vm *ViewModel) GetActiveChatJID() string {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	return vm.ActiveChatJID
}

// StartWatchingMessages subscribes to the message event stream.
// On each event it reloads messages for the active chat and signals refresh.
func (vm *ViewModel) StartWatchingMessages(ctx context.Context) {
	go func() {
		for {
			if err := vm.watchMessages(ctx); err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("message stream error: %v, reconnecting...", err)
				time.Sleep(2 * time.Second)
			}
		}
	}()
}

func (vm *ViewModel) watchMessages(ctx context.Context) error {
	stream, err := vm.client.Message.WatchMessageEvents(ctx, &wppv1.WatchMessageEventsRequest{})
	if err != nil {
		return err
	}
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		chatJID := vm.GetActiveChatJID()
		if chatJID != "" {
			_ = vm.LoadMessages(ctx, chatJID)
		}
	}
}

// StartWatchingChats subscribes to the chat update stream.
// On each event it reloads the chat list and signals refresh.
func (vm *ViewModel) StartWatchingChats(ctx context.Context) {
	go func() {
		for {
			if err := vm.watchChats(ctx); err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("chat stream error: %v, reconnecting...", err)
				time.Sleep(2 * time.Second)
			}
		}
	}()
}

func (vm *ViewModel) watchChats(ctx context.Context) error {
	stream, err := vm.client.Chat.WatchChatUpdates(ctx, &wppv1.WatchChatUpdatesRequest{})
	if err != nil {
		return err
	}
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		_ = vm.LoadChats(ctx)
	}
}
