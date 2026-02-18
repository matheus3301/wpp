package views

import (
	"strings"
	"unicode/utf8"
)

// sanitizeForTerminal removes Unicode codepoints that cause rendering issues
// in tcell/tview. Specifically:
// - Skin tone modifiers (U+1F3FB..U+1F3FF) that create multi-codepoint emoji
// - Zero Width Joiner (U+200D) used in emoji sequences like family/couple emoji
// - Variation Selectors (U+FE00..U+FE0F) that modify preceding characters
// This turns e.g. 👏🏻 into 👏 which renders correctly as a 2-cell-wide character.
func sanitizeForTerminal(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if !isProblematicRune(r) {
			b.WriteRune(r)
		}
		i += size
	}
	return b.String()
}

func isProblematicRune(r rune) bool {
	switch {
	// Skin tone modifiers.
	case r >= 0x1F3FB && r <= 0x1F3FF:
		return true
	// Zero Width Joiner.
	case r == 0x200D:
		return true
	// Variation Selectors.
	case r >= 0xFE00 && r <= 0xFE0F:
		return true
	// Variation Selectors Supplement.
	case r >= 0xE0100 && r <= 0xE01EF:
		return true
	default:
		return false
	}
}
