package session

import (
	"fmt"
	"regexp"
)

var nameRegexp = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)

// ValidateName checks that name conforms to session naming rules.
func ValidateName(name string) error {
	if !nameRegexp.MatchString(name) {
		return fmt.Errorf("invalid session name %q: must match ^[a-z0-9_-]{1,64}$", name)
	}
	return nil
}
