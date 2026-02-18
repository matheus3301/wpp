package api

import (
	"context"
	"time"

	wppv1 "github.com/matheus3301/wpp/gen/wpp/v1"
	"github.com/matheus3301/wpp/internal/bus"
	"github.com/matheus3301/wpp/internal/status"
	"github.com/matheus3301/wpp/internal/store"
	"github.com/matheus3301/wpp/internal/wa"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

// SessionService implements the SessionService gRPC service.
type SessionService struct {
	wppv1.UnimplementedSessionServiceServer

	sessionName string
	startedAt   time.Time
	machine     *status.Machine
	adapter     *wa.Adapter
	bus         *bus.Bus
	db          *store.DB
}

// NewSessionService creates a new session service.
func NewSessionService(sessionName string, machine *status.Machine, adapter *wa.Adapter, b *bus.Bus, db *store.DB) *SessionService {
	return &SessionService{
		sessionName: sessionName,
		startedAt:   time.Now(),
		machine:     machine,
		adapter:     adapter,
		bus:         b,
		db:          db,
	}
}

func (s *SessionService) GetSessionStatus(_ context.Context, _ *wppv1.GetSessionStatusRequest) (*wppv1.GetSessionStatusResponse, error) {
	current := s.machine.Current()

	resp := &wppv1.GetSessionStatusResponse{
		Session:       s.sessionName,
		Status:        stateToProto(current),
		StatusMessage: string(current),
		UptimeMs:      time.Since(s.startedAt).Milliseconds(),
	}

	// Populate phone number from adapter.
	if s.adapter != nil {
		resp.PhoneNumber = s.adapter.PhoneNumber()
	}

	// Populate counts from store.
	if s.db != nil {
		if chatCount, err := s.db.ChatCount(); err == nil {
			resp.ChatCount = int32(chatCount)
		}
		if msgCount, err := s.db.MessageCount(); err == nil {
			resp.MessageCount = int32(msgCount)
		}
	}

	return resp, nil
}

func (s *SessionService) StartAuth(_ *wppv1.StartAuthRequest, stream wppv1.SessionService_StartAuthServer) error {
	if s.adapter == nil {
		return grpcstatus.Errorf(codes.Unavailable, "adapter not initialized")
	}

	authCh, err := s.adapter.StartQRAuth(stream.Context())
	if err != nil {
		return grpcstatus.Errorf(codes.Internal, "start auth: %v", err)
	}

	for evt := range authCh {
		if err := stream.Send(&wppv1.AuthEvent{
			EventType: string(evt.Type),
			QrCode:    evt.QRCode,
			Message:   evt.Message,
		}); err != nil {
			return err
		}
	}

	return nil
}

func (s *SessionService) Logout(ctx context.Context, _ *wppv1.LogoutRequest) (*wppv1.LogoutResponse, error) {
	if s.adapter == nil {
		return nil, grpcstatus.Errorf(codes.Unavailable, "adapter not initialized")
	}
	if err := s.adapter.Logout(ctx); err != nil {
		return nil, grpcstatus.Errorf(codes.Internal, "logout: %v", err)
	}
	return &wppv1.LogoutResponse{Success: true, Message: "logged out"}, nil
}

func (s *SessionService) ListSessions(_ context.Context, _ *wppv1.ListSessionsRequest) (*wppv1.ListSessionsResponse, error) {
	return nil, grpcstatus.Errorf(codes.Unimplemented, "ListSessions not yet implemented")
}

func stateToProto(s status.State) wppv1.SessionStatus {
	switch s {
	case status.Booting:
		return wppv1.SessionStatus_SESSION_STATUS_BOOTING
	case status.AuthRequired:
		return wppv1.SessionStatus_SESSION_STATUS_AUTH_REQUIRED
	case status.Connecting:
		return wppv1.SessionStatus_SESSION_STATUS_CONNECTING
	case status.Syncing:
		return wppv1.SessionStatus_SESSION_STATUS_SYNCING
	case status.Ready:
		return wppv1.SessionStatus_SESSION_STATUS_READY
	case status.Reconnecting:
		return wppv1.SessionStatus_SESSION_STATUS_RECONNECTING
	case status.Degraded:
		return wppv1.SessionStatus_SESSION_STATUS_DEGRADED
	case status.Error:
		return wppv1.SessionStatus_SESSION_STATUS_ERROR
	default:
		return wppv1.SessionStatus_SESSION_STATUS_UNSPECIFIED
	}
}
