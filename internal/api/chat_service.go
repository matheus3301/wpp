package api

import (
	"context"

	"github.com/google/uuid"
	wppv1 "github.com/matheus3301/wpp/gen/wpp/v1"
	"github.com/matheus3301/wpp/internal/bus"
	"github.com/matheus3301/wpp/internal/store"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

// ChatService implements the ChatService gRPC service.
type ChatService struct {
	wppv1.UnimplementedChatServiceServer

	db          *store.DB
	bus         *bus.Bus
	sessionName string
}

// NewChatService creates a new chat service backed by the store.
func NewChatService(db *store.DB, b *bus.Bus, sessionName string) *ChatService {
	return &ChatService{db: db, bus: b, sessionName: sessionName}
}

func (s *ChatService) ListChats(_ context.Context, req *wppv1.ListChatsRequest) (*wppv1.ListChatsResponse, error) {
	limit := 50
	offset := 0
	if req.Pagination != nil && req.Pagination.Limit > 0 {
		limit = int(req.Pagination.Limit)
	}

	chats, err := s.db.ListChats(limit, offset)
	if err != nil {
		return nil, grpcstatus.Errorf(codes.Internal, "list chats: %v", err)
	}

	var pbChats []*wppv1.Chat
	for _, c := range chats {
		pbChats = append(pbChats, chatToProto(&c))
	}

	return &wppv1.ListChatsResponse{
		Chats: pbChats,
		PageInfo: &wppv1.PageInfo{
			HasMore: len(chats) == limit,
		},
	}, nil
}

func (s *ChatService) GetChat(_ context.Context, req *wppv1.GetChatRequest) (*wppv1.GetChatResponse, error) {
	c, err := s.db.GetChat(req.Jid)
	if err != nil {
		return nil, grpcstatus.Errorf(codes.Internal, "get chat: %v", err)
	}
	if c == nil {
		return nil, grpcstatus.Errorf(codes.NotFound, "chat %q not found", req.Jid)
	}
	return &wppv1.GetChatResponse{Chat: chatToProto(c)}, nil
}

func (s *ChatService) WatchChatUpdates(_ *wppv1.WatchChatUpdatesRequest, stream wppv1.ChatService_WatchChatUpdatesServer) error {
	ch, unsub := s.bus.Subscribe("message.", 256)
	defer unsub()

	for {
		select {
		case evt := <-ch:
			if err := stream.Send(&wppv1.EventEnvelope{
				EventId:          uuid.New().String(),
				Session:          s.sessionName,
				OccurredAtUnixMs: evt.Timestamp.UnixMilli(),
				Kind:             evt.Kind,
				PayloadVersion:   1,
			}); err != nil {
				return err
			}
		case <-stream.Context().Done():
			return nil
		}
	}
}

func chatToProto(c *store.Chat) *wppv1.Chat {
	return &wppv1.Chat{
		Jid:                 c.JID,
		Name:                c.Name,
		LastMessagePreview:  c.LastMessagePreview,
		LastMessageAtUnixMs: c.LastMessageAt,
		UnreadCount:         int32(c.UnreadCount),
		IsGroup:             c.IsGroup,
	}
}
