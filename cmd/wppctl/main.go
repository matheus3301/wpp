package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	wppv1 "github.com/matheus3301/wpp/gen/wpp/v1"
	"github.com/matheus3301/wpp/internal/session"
	"github.com/matheus3301/wpp/internal/tui/client"
)

func main() {
	sessionFlag := flag.String("session", "", "session name (overrides config default)")
	jsonFlag := flag.Bool("json", false, "output in JSON format")
	flag.Parse()

	sessionName := session.Resolve(*sessionFlag)
	if err := session.ValidateName(sessionName); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	args := flag.Args()
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	socketPath := session.SocketPath(sessionName)
	c, err := client.New(socketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot connect to daemon for session %q: %v\n", sessionName, err)
		os.Exit(1)
	}
	defer func() { _ = c.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	switch args[0] {
	case "status":
		cmdStatus(ctx, c, *jsonFlag)
	case "auth":
		cmdAuth(ctx, c)
	case "sync":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: wppctl sync <start|stop|status>")
			os.Exit(1)
		}
		cmdSync(ctx, c, args[1], *jsonFlag)
	case "sessions":
		if len(args) >= 2 && args[1] == "list" {
			cmdSessionsList(ctx, c, *jsonFlag)
		} else {
			fmt.Fprintln(os.Stderr, "usage: wppctl sessions list")
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "usage: wppctl [--session <name>] [--json] <command>")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "commands:")
	fmt.Fprintln(os.Stderr, "  status           Show session status")
	fmt.Fprintln(os.Stderr, "  auth             Show auth state")
	fmt.Fprintln(os.Stderr, "  sync start       Start sync")
	fmt.Fprintln(os.Stderr, "  sync stop        Stop sync")
	fmt.Fprintln(os.Stderr, "  sync status      Show sync status")
	fmt.Fprintln(os.Stderr, "  sessions list    List known sessions")
}

func cmdStatus(ctx context.Context, c *client.Client, jsonOut bool) {
	resp, err := c.Session.GetSessionStatus(ctx, &wppv1.GetSessionStatusRequest{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if jsonOut {
		outputJSON(resp)
		return
	}
	fmt.Printf("Session: %s\n", resp.Session)
	fmt.Printf("Status:  %s\n", resp.StatusMessage)
	fmt.Printf("Uptime:  %dms\n", resp.UptimeMs)
}

func cmdAuth(ctx context.Context, c *client.Client) {
	resp, err := c.Session.GetSessionStatus(ctx, &wppv1.GetSessionStatusRequest{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if resp.Status == wppv1.SessionStatus_SESSION_STATUS_AUTH_REQUIRED {
		fmt.Println("Auth required. Use wpptui to scan QR code.")
	} else {
		fmt.Printf("Session authenticated. Status: %s\n", resp.StatusMessage)
	}
}

func cmdSync(ctx context.Context, c *client.Client, subcmd string, jsonOut bool) {
	switch subcmd {
	case "start":
		resp, err := c.Sync.StartSync(ctx, &wppv1.StartSyncRequest{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if jsonOut {
			outputJSON(resp)
			return
		}
		fmt.Printf("Success: %v - %s\n", resp.Success, resp.Message)
	case "stop":
		resp, err := c.Sync.StopSync(ctx, &wppv1.StopSyncRequest{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if jsonOut {
			outputJSON(resp)
			return
		}
		fmt.Printf("Success: %v - %s\n", resp.Success, resp.Message)
	case "status":
		resp, err := c.Sync.GetSyncStatus(ctx, &wppv1.GetSyncStatusRequest{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if jsonOut {
			outputJSON(resp)
			return
		}
		fmt.Printf("Syncing: %v\n", resp.Syncing)
		fmt.Printf("Messages synced: %d\n", resp.MessagesSynced)
		fmt.Printf("Chats synced: %d\n", resp.ChatsSynced)
	default:
		fmt.Fprintf(os.Stderr, "unknown sync subcommand: %s\n", subcmd)
		os.Exit(1)
	}
}

func cmdSessionsList(ctx context.Context, c *client.Client, jsonOut bool) {
	resp, err := c.Session.ListSessions(ctx, &wppv1.ListSessionsRequest{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if jsonOut {
		outputJSON(resp)
		return
	}
	if len(resp.Sessions) == 0 {
		fmt.Println("No sessions found.")
		return
	}
	for _, s := range resp.Sessions {
		running := "stopped"
		if s.DaemonRunning {
			running = "running"
		}
		fmt.Printf("%-20s %s (%s)\n", s.Name, s.Path, running)
	}
}

func outputJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "json encode error: %v\n", err)
	}
}
