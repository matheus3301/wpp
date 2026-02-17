package lock

import (
	"errors"
	"os"
	"testing"
)

func TestAcquireAndRelease(t *testing.T) {
	tmpDir := t.TempDir()

	l, err := Acquire(tmpDir)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}

	// Verify lock file exists and contains PID.
	data, err := os.ReadFile(tmpDir + "/LOCK")
	if err != nil {
		t.Fatalf("read lock file: %v", err)
	}
	if len(data) == 0 {
		t.Error("lock file is empty")
	}

	if err := l.Release(); err != nil {
		t.Errorf("Release() error = %v", err)
	}
}

func TestDoubleAcquireFails(t *testing.T) {
	tmpDir := t.TempDir()

	l1, err := Acquire(tmpDir)
	if err != nil {
		t.Fatalf("first Acquire() error = %v", err)
	}
	defer func() { _ = l1.Release() }()

	_, err = Acquire(tmpDir)
	if err == nil {
		t.Fatal("second Acquire() should fail")
	}

	var lockErr *LockHeldError
	if !errors.As(err, &lockErr) {
		t.Errorf("expected LockHeldError, got %T: %v", err, err)
	}
}

func TestReleaseNil(t *testing.T) {
	var l *Lock
	if err := l.Release(); err != nil {
		t.Errorf("nil Release() error = %v", err)
	}
}

func TestReleaseIdempotent(t *testing.T) {
	tmpDir := t.TempDir()

	l, err := Acquire(tmpDir)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}

	if err := l.Release(); err != nil {
		t.Errorf("first Release() error = %v", err)
	}
	if err := l.Release(); err != nil {
		t.Errorf("second Release() error = %v", err)
	}
}
