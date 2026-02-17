package lock

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// LockHeldError is returned when another process holds the session lock.
type LockHeldError struct {
	PID  int
	Path string
}

func (e *LockHeldError) Error() string {
	return fmt.Sprintf("session lock held by PID %d (%s)", e.PID, e.Path)
}

// Lock represents an acquired session lock file.
type Lock struct {
	file *os.File
	path string
}

// Acquire attempts to acquire an exclusive lock on the session directory.
// Returns LockHeldError if another process already holds it.
func Acquire(sessionDir string) (*Lock, error) {
	lockPath := filepath.Join(sessionDir, "LOCK")

	if err := os.MkdirAll(sessionDir, 0700); err != nil {
		return nil, fmt.Errorf("create session dir: %w", err)
	}

	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}

	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		// Read existing PID from file for diagnostics.
		data, _ := os.ReadFile(lockPath)
		pid := parsePID(string(data))
		_ = f.Close()
		return nil, &LockHeldError{PID: pid, Path: lockPath}
	}

	// Write PID + timestamp.
	if err := f.Truncate(0); err != nil {
		_ = f.Close()
		return nil, err
	}
	if _, err := f.Seek(0, 0); err != nil {
		_ = f.Close()
		return nil, err
	}
	content := fmt.Sprintf("pid=%d\ntime=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339))
	if _, err := f.WriteString(content); err != nil {
		_ = f.Close()
		return nil, err
	}

	return &Lock{file: f, path: lockPath}, nil
}

// Release releases the lock. Safe to call on nil receiver.
func (l *Lock) Release() error {
	if l == nil || l.file == nil {
		return nil
	}
	// Remove lock file before closing to avoid stale files.
	_ = os.Remove(l.path)
	err := l.file.Close()
	l.file = nil
	return err
}

func parsePID(content string) int {
	for _, line := range strings.Split(content, "\n") {
		if after, ok := strings.CutPrefix(line, "pid="); ok {
			pid, _ := strconv.Atoi(after)
			return pid
		}
	}
	return 0
}
