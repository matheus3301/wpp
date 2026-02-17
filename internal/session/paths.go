package session

import (
	"os"
	"path/filepath"
)

// BaseDir returns ~/.wpp.
func BaseDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".wpp")
}

// Dir returns the session-specific directory.
func Dir(name string) string {
	return filepath.Join(BaseDir(), "sessions", name)
}

// SocketPath returns the UDS socket path for a session.
func SocketPath(name string) string {
	return filepath.Join(Dir(name), "daemon.sock")
}

// LockPath returns the lock file path for a session.
func LockPath(name string) string {
	return filepath.Join(Dir(name), "LOCK")
}

// SessionDBPath returns the whatsmeow session.db path.
func SessionDBPath(name string) string {
	return filepath.Join(Dir(name), "session.db")
}

// AppDBPath returns the app-owned wpp.db path.
func AppDBPath(name string) string {
	return filepath.Join(Dir(name), "wpp.db")
}

// LogDir returns the log directory for a session.
func LogDir(name string) string {
	return filepath.Join(Dir(name), "logs")
}

// LogPath returns the daemon log file path.
func LogPath(name string) string {
	return filepath.Join(LogDir(name), "wppd.log")
}

// ConfigPath returns the global config file path.
func ConfigPath() string {
	return filepath.Join(BaseDir(), "config.toml")
}

// EnsureDir creates the session directory tree with proper permissions.
func EnsureDir(name string) error {
	dirs := []string{
		Dir(name),
		LogDir(name),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0700); err != nil {
			return err
		}
	}
	return nil
}
