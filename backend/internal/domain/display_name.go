package domain

import (
	"strings"

	"github.com/rivo/uniseg"
	"github.com/rotisserie/eris"
)

const displayNameMax = 50

var (
	ErrDisplayNameRequired = eris.New("user: display name is required")
	ErrDisplayNameTooLong  = eris.Errorf("user: display name exceeds %d characters", displayNameMax)
)

// DisplayName is the user-chosen profile display name, trimmed, 1..50 graphemes.
type DisplayName string

// ParseDisplayName trims surrounding whitespace from s, counts grapheme clusters,
// and returns a validated DisplayName or a sentinel error.
func ParseDisplayName(s string) (DisplayName, error) {
	trimmed := strings.TrimSpace(s)
	n := uniseg.GraphemeClusterCount(trimmed)
	if n < 1 {
		return "", ErrDisplayNameRequired
	}
	if n > displayNameMax {
		return "", ErrDisplayNameTooLong
	}
	return DisplayName(trimmed), nil
}
