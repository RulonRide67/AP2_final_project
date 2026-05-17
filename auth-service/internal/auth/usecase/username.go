package usecase

import (
	"strings"
)

// BuildUsername derives a unique login name from name parts or email local-part.
func BuildUsername(firstName, lastName, email string) string {
	first := strings.TrimSpace(firstName)
	last := strings.TrimSpace(lastName)

	switch {
	case first != "" && last != "":
		return strings.ToLower(first + "." + last)
	case first != "":
		return strings.ToLower(first)
	case last != "":
		return strings.ToLower(last)
	}

	if at := strings.Index(email, "@"); at > 0 {
		return strings.ToLower(email[:at])
	}
	return strings.ToLower(email)
}
