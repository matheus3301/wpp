package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	wppv1 "github.com/matheus3301/wpp/gen/wpp/v1"
	"github.com/matheus3301/wpp/internal/session"
	"github.com/matheus3301/wpp/internal/tui"
	"github.com/matheus3301/wpp/internal/tui/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	sessionFlag := flag.String("session", "", "session name (overrides config default)")
	flag.Parse()

	sessionName := session.Resolve(*sessionFlag)
	if err := session.ValidateName(sessionName); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	socketPath := session.SocketPath(sessionName)

	// Probe daemon health; auto-start if needed.
	if !probeDaemon(socketPath) {
		fmt.Fprintf(os.Stderr, "daemon not running for session %q, starting...\n", sessionName)
		if err := startDaemon(sessionName); err != nil {
			fmt.Fprintf(os.Stderr, "failed to start daemon: %v\n", err)
			os.Exit(1)
		}
		if !waitForDaemon(socketPath, 10*time.Second) {
			fmt.Fprintf(os.Stderr, "daemon did not become ready\n")
			os.Exit(1)
		}
	}

	c, err := client.New(socketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect to daemon: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = c.Close() }()

	app := tui.NewApp(c, sessionName)
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// probeDaemon checks if a daemon is running and responsive on the socket.
func probeDaemon(socketPath string) bool {
	conn, err := grpc.NewClient(
		"unix://"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return false
	}
	defer func() { _ = conn.Close() }()

	c := wppv1.NewSessionServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = c.GetSessionStatus(ctx, &wppv1.GetSessionStatusRequest{})
	return err == nil
}

func startDaemon(sessionName string) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	wppd := filepath.Join(filepath.Dir(executable), "wppd")

	if _, err := os.Stat(wppd); err != nil {
		wppd = "wppd"
	}

	cmd := exec.Command(wppd, "--session", sessionName)
	// Inherit stderr so daemon startup errors are visible.
	cmd.Stderr = os.Stderr
	return cmd.Start()
}

// waitForDaemon polls the daemon with a real gRPC health check (not just socket connect).
func waitForDaemon(socketPath string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if probeDaemon(socketPath) {
			return true
		}
		time.Sleep(300 * time.Millisecond)
	}
	return false
}
