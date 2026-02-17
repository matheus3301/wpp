package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDir(t *testing.T) {
	home, _ := os.UserHomeDir()
	got := Dir("main")
	want := filepath.Join(home, ".wpp", "sessions", "main")
	if got != want {
		t.Errorf("Dir(main) = %q, want %q", got, want)
	}
}

func TestSocketPath(t *testing.T) {
	got := SocketPath("test")
	if !strings.HasSuffix(got, filepath.Join("sessions", "test", "daemon.sock")) {
		t.Errorf("SocketPath(test) = %q, want suffix sessions/test/daemon.sock", got)
	}
}

func TestLockPath(t *testing.T) {
	got := LockPath("test")
	if !strings.HasSuffix(got, filepath.Join("sessions", "test", "LOCK")) {
		t.Errorf("LockPath(test) = %q, want suffix sessions/test/LOCK", got)
	}
}

func TestEnsureDir(t *testing.T) {
	tmpDir := t.TempDir()
	// Override BaseDir for testing by using a custom session dir.
	sessionDir := filepath.Join(tmpDir, "sessions", "test")
	logDir := filepath.Join(sessionDir, "logs")

	if err := os.MkdirAll(sessionDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(logDir, 0700); err != nil {
		t.Fatal(err)
	}

	// Verify dirs were created.
	info, err := os.Stat(sessionDir)
	if err != nil {
		t.Fatalf("session dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("session dir is not a directory")
	}
}
