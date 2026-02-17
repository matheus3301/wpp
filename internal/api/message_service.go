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

// MessageService implements the MessageService gRPC service.
type MessageService struct {
	wppv1.UnimplementedMessageServiceServer

	db  *store.DB
	bus *bus.Bus
}

// NewMessageService creates a new message service backed by the store.
func NewMessageService(db *store.DB, b *bus.Bus) *MessageService {
	return &MessageService{db: db, bus: b}
}

func (s *MessageService) ListMessages(_ context.Context, req *wppv1.ListMessagesRequest) (*wppv1.ListMessagesResponse, error) {
	limit := 50
	var beforeTs int64
	if req.Pagination != nil {
		if req.Pagination.Limit > 0 {
			limit = int(req.Pagination.Limit)
		}
	}

	msgs, err := s.db.ListMessages(req.ChatJid, beforeTs, limit)
	if err != nil {
		return nil, grpcstatus.Errorf(codes.Internal, "list messages: %v", err)
	}

	var pbMsgs []*wppv1.Message
	for _, m := range msgs {
		pbMsgs = append(pbMsgs, messageToProto(&m))
	}

	return &wppv1.ListMessagesResponse{
		Messages: pbMsgs,
		PageInfo: &wppv1.PageInfo{
			HasMore: len(msgs) == limit,
		},
	}, nil
}

func (s *MessageService) SearchMessages(_ context.Context, req *wppv1.SearchMessagesRequest) (*wppv1.SearchMessagesResponse, error) {
	limit := 50
	if req.Pagination != nil && req.Pagination.Limit > 0 {
		limit = int(req.Pagination.Limit)
	}

	results, err := s.db.SearchMessages(req.Query, req.ChatJid, limit)
	if err != nil {
		return nil, grpcstatus.Errorf(codes.Internal, "search messages: %v", err)
	}

	var pbResults []*wppv1.SearchResult
	for _, r := range results {
		pbResults = append(pbResults, &wppv1.SearchResult{
			Message: messageToProto(&r.Message),
			Snippet: r.Snippet,
		})
	}

	return &wppv1.SearchMessagesResponse{
		Results: pbResults,
		PageInfo: &wppv1.PageInfo{
			HasMore: len(results) == limit,
		},
	}, nil
}

func (s *MessageService) SendText(_ context.Context, req *wppv1.SendTextRequest) (*wppv1.SendTextResponse, error) {
	if err := s.db.QueueOutbox(req.ClientMsgId, req.ChatJid, req.Text); err != nil {
		return nil, grpcstatus.Errorf(codes.Internal, "queue outbox: %v", err)
	}
	return &wppv1.SendTextResponse{Accepted: true, Message: "queued"}, nil
}

func (s *MessageService) WatchMessageEvents(req *wppv1.WatchMessageEventsRequest, stream wppv1.MessageService_WatchMessageEventsServer) error {
	ch, unsub := s.bus.Subscribe("message.", 64)
	defer unsub()

	for {
		select {
		case evt := <-ch:
			if err := stream.Send(&wppv1.EventEnvelope{
				EventId:          uuid.New().String(),
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

func messageToProto(m *store.Message) *wppv1.Message {
	return &wppv1.Message{
		Id:              m.MsgID,
		ChatJid:         m.ChatJID,
		SenderJid:       m.SenderJID,
		SenderName:      m.SenderName,
		Body:            m.Body,
		TimestampUnixMs: m.Timestamp,
		FromMe:          m.FromMe,
		MessageType:     m.MessageType,
		Status:          m.Status,
	}
}
