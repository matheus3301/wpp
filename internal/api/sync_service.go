package api

import (
	"context"
	"time"

	"github.com/google/uuid"
	wppv1 "github.com/matheus3301/wpp/gen/wpp/v1"
	"github.com/matheus3301/wpp/internal/bus"
	"github.com/matheus3301/wpp/internal/wa"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// SyncService implements the SyncService gRPC service.
type SyncService struct {
	wppv1.UnimplementedSyncServiceServer

	adapter *wa.Adapter
	bus     *bus.Bus
	syncing bool
}

// NewSyncService creates a new sync service.
func NewSyncService(adapter *wa.Adapter, b *bus.Bus) *SyncService {
	return &SyncService{
		adapter: adapter,
		bus:     b,
	}
}

func (s *SyncService) GetSyncStatus(_ context.Context, _ *wppv1.GetSyncStatusRequest) (*wppv1.GetSyncStatusResponse, error) {
	return &wppv1.GetSyncStatusResponse{
		Syncing: s.syncing,
	}, nil
}

func (s *SyncService) StartSync(_ context.Context, _ *wppv1.StartSyncRequest) (*wppv1.StartSyncResponse, error) {
	if s.adapter == nil {
		return nil, grpcstatus.Errorf(codes.Unavailable, "adapter not initialized")
	}
	if s.syncing {
		return &wppv1.StartSyncResponse{Success: true, Message: "already syncing"}, nil
	}
	if err := s.adapter.Connect(); err != nil {
		return nil, grpcstatus.Errorf(codes.Internal, "connect: %v", err)
	}
	s.syncing = true
	return &wppv1.StartSyncResponse{Success: true, Message: "sync started"}, nil
}

func (s *SyncService) StopSync(_ context.Context, _ *wppv1.StopSyncRequest) (*wppv1.StopSyncResponse, error) {
	if s.adapter == nil {
		return nil, grpcstatus.Errorf(codes.Unavailable, "adapter not initialized")
	}
	s.adapter.Disconnect()
	s.syncing = false
	return &wppv1.StopSyncResponse{Success: true, Message: "sync stopped"}, nil
}

func (s *SyncService) WatchSyncEvents(_ *wppv1.WatchSyncEventsRequest, stream wppv1.SyncService_WatchSyncEventsServer) error {
	ch, unsub := s.bus.Subscribe("sync.", 64)
	defer unsub()

	for {
		select {
		case evt := <-ch:
			payload, _ := proto.Marshal(&wppv1.SyncConnected{})
			if err := stream.Send(&wppv1.EventEnvelope{
				EventId:          uuid.New().String(),
				Session:          "",
				OccurredAtUnixMs: evt.Timestamp.UnixMilli(),
				Kind:             evt.Kind,
				PayloadVersion:   1,
				Payload:          payload,
			}); err != nil {
				return err
			}
		case <-stream.Context().Done():
			return nil
		}
	}
}

// SetSyncing is used internally to update sync state from lifecycle hooks.
func (s *SyncService) SetSyncing(syncing bool) {
	s.syncing = syncing
}

// LastSyncAt returns the timestamp for API responses.
func (s *SyncService) LastSyncAt() int64 {
	if s.syncing {
		return time.Now().UnixMilli()
	}
	return 0
}
