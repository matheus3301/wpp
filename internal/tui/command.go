package tui

import "strings"

// Command represents a parsed command.
type Command struct {
	Name string
	Args string
}

// ParseCommand parses a command string (without the leading ':').
func ParseCommand(input string) Command {
	input = strings.TrimSpace(input)
	parts := strings.SplitN(input, " ", 2)
	cmd := Command{Name: strings.ToLower(parts[0])}
	if len(parts) > 1 {
		cmd.Args = strings.TrimSpace(parts[1])
	}
	return cmd
}
