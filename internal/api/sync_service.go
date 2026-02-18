package api

import (
	"context"

	"github.com/google/uuid"
	wppv1 "github.com/matheus3301/wpp/gen/wpp/v1"
	"github.com/matheus3301/wpp/internal/bus"
	"github.com/matheus3301/wpp/internal/status"
	"github.com/matheus3301/wpp/internal/wa"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// SyncService implements the SyncService gRPC service.
type SyncService struct {
	wppv1.UnimplementedSyncServiceServer

	adapter     *wa.Adapter
	bus         *bus.Bus
	machine     *status.Machine
	sessionName string
}

// NewSyncService creates a new sync service.
func NewSyncService(adapter *wa.Adapter, b *bus.Bus, machine *status.Machine, sessionName string) *SyncService {
	return &SyncService{
		adapter:     adapter,
		bus:         b,
		machine:     machine,
		sessionName: sessionName,
	}
}

func (s *SyncService) GetSyncStatus(_ context.Context, _ *wppv1.GetSyncStatusRequest) (*wppv1.GetSyncStatusResponse, error) {
	current := s.machine.Current()
	syncing := current == status.Syncing || current == status.Ready
	return &wppv1.GetSyncStatusResponse{
		Syncing: syncing,
	}, nil
}

func (s *SyncService) StartSync(_ context.Context, _ *wppv1.StartSyncRequest) (*wppv1.StartSyncResponse, error) {
	if s.adapter == nil {
		return nil, grpcstatus.Errorf(codes.Unavailable, "adapter not initialized")
	}
	current := s.machine.Current()
	if current == status.Syncing || current == status.Ready {
		return &wppv1.StartSyncResponse{Success: true, Message: "already syncing"}, nil
	}
	if err := s.adapter.Connect(); err != nil {
		return nil, grpcstatus.Errorf(codes.Internal, "connect: %v", err)
	}
	return &wppv1.StartSyncResponse{Success: true, Message: "sync started"}, nil
}

func (s *SyncService) StopSync(_ context.Context, _ *wppv1.StopSyncRequest) (*wppv1.StopSyncResponse, error) {
	if s.adapter == nil {
		return nil, grpcstatus.Errorf(codes.Unavailable, "adapter not initialized")
	}
	s.adapter.Disconnect()
	return &wppv1.StopSyncResponse{Success: true, Message: "sync stopped"}, nil
}

func (s *SyncService) WatchSyncEvents(_ *wppv1.WatchSyncEventsRequest, stream wppv1.SyncService_WatchSyncEventsServer) error {
	ch, unsub := s.bus.Subscribe("sync.", 256)
	defer unsub()

	for {
		select {
		case evt := <-ch:
			payload, _ := marshalSyncPayload(evt.Kind)
			if err := stream.Send(&wppv1.EventEnvelope{
				EventId:          uuid.New().String(),
				Session:          s.sessionName,
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

// marshalSyncPayload returns the correct proto payload for each sync event kind.
func marshalSyncPayload(kind string) ([]byte, error) {
	var msg proto.Message
	switch kind {
	case "sync.connected":
		msg = &wppv1.SyncConnected{}
	case "sync.disconnected":
		msg = &wppv1.SyncDisconnected{}
	case "sync.reconnecting":
		msg = &wppv1.SyncReconnecting{}
	case "sync.history_batch":
		msg = &wppv1.SyncHistoryBatch{}
	case "sync.connecting":
		msg = &wppv1.SyncConnecting{}
	case "sync.degraded":
		msg = &wppv1.SyncDegraded{}
	default:
		msg = &wppv1.SyncConnected{}
	}
	return proto.Marshal(msg)
}
