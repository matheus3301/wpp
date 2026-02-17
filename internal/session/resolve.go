package session

import "github.com/matheus3301/wpp/internal/config"

const DefaultSessionName = "main"

// Resolve determines the active session name using precedence:
// 1. flagOverride (--session flag)
// 2. config.toml default_session
// 3. "main"
func Resolve(flagOverride string) string {
	if flagOverride != "" {
		return flagOverride
	}
	cfg, err := config.Load(ConfigPath())
	if err == nil && cfg.DefaultSession != "" {
		return cfg.DefaultSession
	}
	return DefaultSessionName
}
