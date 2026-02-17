package client

import (
	"fmt"

	wppv1 "github.com/matheus3301/wpp/gen/wpp/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client wraps gRPC connections to the daemon.
type Client struct {
	conn    *grpc.ClientConn
	Session wppv1.SessionServiceClient
	Sync    wppv1.SyncServiceClient
	Chat    wppv1.ChatServiceClient
	Message wppv1.MessageServiceClient
}

// New dials the daemon's Unix domain socket and returns typed service clients.
func New(socketPath string) (*Client, error) {
	conn, err := grpc.NewClient(
		"unix://"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("dial daemon: %w", err)
	}

	return &Client{
		conn:    conn,
		Session: wppv1.NewSessionServiceClient(conn),
		Sync:    wppv1.NewSyncServiceClient(conn),
		Chat:    wppv1.NewChatServiceClient(conn),
		Message: wppv1.NewMessageServiceClient(conn),
	}, nil
}

// Close closes the gRPC connection.
func (c *Client) Close() error {
	return c.conn.Close()
}
