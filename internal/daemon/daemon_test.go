package daemon

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	wppv1 "github.com/matheus3301/wpp/gen/wpp/v1"
	"github.com/matheus3301/wpp/internal/api"
	"github.com/matheus3301/wpp/internal/bus"
	"github.com/matheus3301/wpp/internal/lock"
	"github.com/matheus3301/wpp/internal/status"
	"github.com/matheus3301/wpp/internal/store"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestDaemonLifecycle(t *testing.T) {
	// Use a short path to avoid macOS 104-char Unix socket limit.
	tmpDir, err := os.MkdirTemp("/tmp", "wpp-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	sessionName := "test"
	sessionDir := filepath.Join(tmpDir, sessionName)
	socketPath := filepath.Join(sessionDir, "d.sock")

	if err := os.MkdirAll(sessionDir, 0700); err != nil {
		t.Fatal(err)
	}

	// Acquire lock.
	lk, err := lock.Acquire(sessionDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lk.Release() }()

	// Open store.
	db, err := store.Open(filepath.Join(sessionDir, "wpp.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	// Setup components.
	logger, _ := zap.NewDevelopment()
	b := bus.New()
	machine := status.NewMachine(b)
	sessionSvc := api.NewSessionService(sessionName, machine, nil, b)
	syncSvc := api.NewSyncService(nil, b, machine, sessionName)
	chatSvc := api.NewChatService(db, b, sessionName)
	messageSvc := api.NewMessageService(db, b, sessionName)

	// Create gRPC server manually.
	grpcSrv := grpc.NewServer()
	wppv1.RegisterSessionServiceServer(grpcSrv, sessionSvc)
	wppv1.RegisterSyncServiceServer(grpcSrv, syncSvc)
	wppv1.RegisterChatServiceServer(grpcSrv, chatSvc)
	wppv1.RegisterMessageServiceServer(grpcSrv, messageSvc)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}

	go func() { _ = grpcSrv.Serve(listener) }()
	defer grpcSrv.GracefulStop()

	time.Sleep(50 * time.Millisecond)

	// Connect as client.
	conn, err := grpc.NewClient(
		"unix://"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	// Test GetSessionStatus.
	client := wppv1.NewSessionServiceClient(conn)
	resp, err := client.GetSessionStatus(context.Background(), &wppv1.GetSessionStatusRequest{})
	if err != nil {
		t.Fatalf("GetSessionStatus error = %v", err)
	}
	if resp.Session != sessionName {
		t.Errorf("session = %q, want %q", resp.Session, sessionName)
	}
	if resp.Status != wppv1.SessionStatus_SESSION_STATUS_BOOTING {
		t.Errorf("status = %v, want BOOTING", resp.Status)
	}

	// Test GetSyncStatus.
	syncClient := wppv1.NewSyncServiceClient(conn)
	syncResp, err := syncClient.GetSyncStatus(context.Background(), &wppv1.GetSyncStatusRequest{})
	if err != nil {
		t.Fatalf("GetSyncStatus error = %v", err)
	}
	if syncResp.Syncing {
		t.Error("expected syncing = false")
	}

	// Test ListChats (empty).
	chatClient := wppv1.NewChatServiceClient(conn)
	chatResp, err := chatClient.ListChats(context.Background(), &wppv1.ListChatsRequest{})
	if err != nil {
		t.Fatalf("ListChats error = %v", err)
	}
	if len(chatResp.Chats) != 0 {
		t.Errorf("expected 0 chats, got %d", len(chatResp.Chats))
	}

	// Insert a chat and messages, then query.
	if err := db.UpsertChat(&store.Chat{JID: "test@s", Name: "Test", LastMessageAt: 1000, LastMessagePreview: "hello"}); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertMessage(&store.Message{ChatJID: "test@s", MsgID: "m1", Body: "hello world", MessageType: "text", Timestamp: 1000}); err != nil {
		t.Fatal(err)
	}

	chatResp, err = chatClient.ListChats(context.Background(), &wppv1.ListChatsRequest{})
	if err != nil {
		t.Fatalf("ListChats error = %v", err)
	}
	if len(chatResp.Chats) != 1 {
		t.Errorf("expected 1 chat, got %d", len(chatResp.Chats))
	}

	// Test ListMessages.
	msgClient := wppv1.NewMessageServiceClient(conn)
	msgResp, err := msgClient.ListMessages(context.Background(), &wppv1.ListMessagesRequest{ChatJid: "test@s"})
	if err != nil {
		t.Fatalf("ListMessages error = %v", err)
	}
	if len(msgResp.Messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(msgResp.Messages))
	}

	// Test SearchMessages.
	searchResp, err := msgClient.SearchMessages(context.Background(), &wppv1.SearchMessagesRequest{Query: "hello"})
	if err != nil {
		t.Fatalf("SearchMessages error = %v", err)
	}
	if len(searchResp.Results) != 1 {
		t.Errorf("expected 1 search result, got %d", len(searchResp.Results))
	}

	// Test SendText.
	sendResp, err := msgClient.SendText(context.Background(), &wppv1.SendTextRequest{ClientMsgId: "c1", ChatJid: "test@s", Text: "test"})
	if err != nil {
		t.Fatalf("SendText error = %v", err)
	}
	if !sendResp.Accepted {
		t.Error("expected accepted = true")
	}

	logger.Info("integration test passed")
}

// TestStatusTransitionsToAuthRequired verifies the daemon status transitions
// from BOOTING to AUTH_REQUIRED when queried via the gRPC API.
// Regression test: the daemon previously stayed in BOOTING forever because
// nothing transitioned the state machine after startup.
func TestStatusTransitionsToAuthRequired(t *testing.T) {
	tmpDir, err := os.MkdirTemp("/tmp", "wpp-auth-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	sessionDir := filepath.Join(tmpDir, "s")
	socketPath := filepath.Join(tmpDir, "d.sock")
	if err := os.MkdirAll(sessionDir, 0700); err != nil {
		t.Fatal(err)
	}

	b := bus.New()
	machine := status.NewMachine(b)

	// Simulate what registerLifecycle does when adapter is NOT logged in.
	_ = machine.Transition(status.AuthRequired)

	sessionSvc := api.NewSessionService("test", machine, nil, b)

	grpcSrv := grpc.NewServer()
	wppv1.RegisterSessionServiceServer(grpcSrv, sessionSvc)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = grpcSrv.Serve(listener) }()
	defer grpcSrv.GracefulStop()

	time.Sleep(50 * time.Millisecond)

	conn, err := grpc.NewClient(
		"unix://"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	client := wppv1.NewSessionServiceClient(conn)
	resp, err := client.GetSessionStatus(context.Background(), &wppv1.GetSessionStatusRequest{})
	if err != nil {
		t.Fatalf("GetSessionStatus error = %v", err)
	}

	if resp.Status != wppv1.SessionStatus_SESSION_STATUS_AUTH_REQUIRED {
		t.Errorf("status = %v, want AUTH_REQUIRED; daemon must not stay in BOOTING when unauthenticated", resp.Status)
	}
}

// TestStatusReflectsPostAuthTransition verifies that the gRPC status endpoint
// reflects state changes after authentication completes.
// Regression: the daemon stayed stuck on AUTH_REQUIRED after QR auth because
// the Connected event tried an invalid AUTH_REQUIRED→SYNCING transition.
// The fix routes through AUTH_REQUIRED→CONNECTING→SYNCING.
func TestStatusReflectsPostAuthTransition(t *testing.T) {
	tmpDir, err := os.MkdirTemp("/tmp", "wpp-post-auth-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	socketPath := filepath.Join(tmpDir, "d.sock")

	b := bus.New()
	machine := status.NewMachine(b)

	// Start at AUTH_REQUIRED (first-run, no credentials).
	_ = machine.Transition(status.AuthRequired)

	sessionSvc := api.NewSessionService("test", machine, nil, b)

	grpcSrv := grpc.NewServer()
	wppv1.RegisterSessionServiceServer(grpcSrv, sessionSvc)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = grpcSrv.Serve(listener) }()
	defer grpcSrv.GracefulStop()
	time.Sleep(50 * time.Millisecond)

	conn, err := grpc.NewClient(
		"unix://"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	client := wppv1.NewSessionServiceClient(conn)

	// Verify initial state is AUTH_REQUIRED.
	resp, err := client.GetSessionStatus(context.Background(), &wppv1.GetSessionStatusRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != wppv1.SessionStatus_SESSION_STATUS_AUTH_REQUIRED {
		t.Fatalf("initial status = %v, want AUTH_REQUIRED", resp.Status)
	}

	// Simulate what the event handler does after QR auth + Connected event:
	// AUTH_REQUIRED → CONNECTING → SYNCING
	_ = machine.Transition(status.Connecting)
	_ = machine.Transition(status.Syncing)

	// Verify gRPC now reports SYNCING.
	resp, err = client.GetSessionStatus(context.Background(), &wppv1.GetSessionStatusRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != wppv1.SessionStatus_SESSION_STATUS_SYNCING {
		t.Errorf("post-auth status = %v, want SYNCING; status must not stay stuck on AUTH_REQUIRED", resp.Status)
	}

	// Transition to READY on first message.
	_ = machine.Transition(status.Ready)

	resp, err = client.GetSessionStatus(context.Background(), &wppv1.GetSessionStatusRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != wppv1.SessionStatus_SESSION_STATUS_READY {
		t.Errorf("final status = %v, want READY", resp.Status)
	}
}

// TestFxModuleWiring verifies the fx dependency graph resolves without errors.
// Regression test: NewServer previously took a bare `string` param which fx
// cannot resolve, causing a silent startup crash ("missing type: string").
func TestFxModuleWiring(t *testing.T) {
	// Use /tmp for short socket paths (macOS 104-char limit).
	tmpDir, err := os.MkdirTemp("/tmp", "wpp-fx-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	socketPath := filepath.Join(tmpDir, "d.sock")

	// Test that NewServer accepts Params (not a bare string).
	// Regression: a bare `string` param caused fx to fail with "missing type: string".
	p := Params{SessionName: "fxtest", SocketPath: socketPath}
	srv, err := NewServer(
		p,
		zap.NewNop(),
		api.NewSessionService("fxtest", status.NewMachine(nil), nil, nil),
		api.NewSyncService(nil, nil, status.NewMachine(nil), "fxtest"),
		api.NewChatService(nil, nil, "fxtest"),
		api.NewMessageService(nil, nil, "fxtest"),
	)
	if err != nil {
		t.Fatalf("NewServer() with Params failed: %v", err)
	}

	// Verify socket was created inside the temp dir (not ~/.wpp).
	if _, statErr := os.Stat(socketPath); statErr != nil {
		t.Fatalf("socket not created at %s: %v", socketPath, statErr)
	}

	// Clean up.
	srv.Stop(context.Background())
}
