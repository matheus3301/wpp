package daemon

import (
	"context"
	"fmt"
	"net"
	"os"

	wppv1 "github.com/matheus3301/wpp/gen/wpp/v1"
	"github.com/matheus3301/wpp/internal/api"
	"github.com/matheus3301/wpp/internal/session"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// Server manages the gRPC server lifecycle for a session daemon.
type Server struct {
	grpcServer *grpc.Server
	listener   net.Listener
	socketPath string
	logger     *zap.Logger
}

// NewServer creates a gRPC server bound to the session's Unix domain socket.
func NewServer(
	p Params,
	logger *zap.Logger,
	sessionSvc *api.SessionService,
	syncSvc *api.SyncService,
	chatSvc *api.ChatService,
	messageSvc *api.MessageService,
) (*Server, error) {
	sessionName := p.SessionName
	socketPath := p.SocketPath
	if socketPath == "" {
		socketPath = session.SocketPath(sessionName)
	}

	// Clean stale socket if it exists.
	if _, err := os.Stat(socketPath); err == nil {
		_ = os.Remove(socketPath)
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("listen unix socket: %w", err)
	}

	// Set socket permissions to 0600.
	if err := os.Chmod(socketPath, 0600); err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("chmod socket: %w", err)
	}

	srv := grpc.NewServer()
	wppv1.RegisterSessionServiceServer(srv, sessionSvc)
	wppv1.RegisterSyncServiceServer(srv, syncSvc)
	wppv1.RegisterChatServiceServer(srv, chatSvc)
	wppv1.RegisterMessageServiceServer(srv, messageSvc)

	return &Server{
		grpcServer: srv,
		listener:   listener,
		socketPath: socketPath,
		logger:     logger,
	}, nil
}

// Start begins serving gRPC requests. Blocks until stopped.
func (s *Server) Start() error {
	s.logger.Info("gRPC server starting", zap.String("socket", s.socketPath))
	return s.grpcServer.Serve(s.listener)
}

// Stop performs a graceful shutdown and removes the socket file.
func (s *Server) Stop(_ context.Context) {
	s.logger.Info("gRPC server stopping")
	s.grpcServer.GracefulStop()
	_ = os.Remove(s.socketPath)
}
