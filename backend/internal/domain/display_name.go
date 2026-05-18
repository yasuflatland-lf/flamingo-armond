package domain

import (
	"strings"

	"github.com/rivo/uniseg"
	"github.com/rotisserie/eris"
)

const DisplayNameMax = 50

var (
	ErrDisplayNameRequired = eris.New("user: display name is required")
	ErrDisplayNameTooLong  = eris.Errorf("user: display name exceeds %d characters", DisplayNameMax)
)

// DisplayName is the user-chosen profile display name, trimmed, 1..DisplayNameMax graphemes.
// The zero value (DisplayName("")) is invalid; use ParseDisplayName to construct.
type DisplayName string

// String returns the underlying string value.
func (d DisplayName) String() string { return string(d) }

// ParseDisplayName trims surrounding whitespace from s, counts grapheme clusters,
// and returns a validated DisplayName or a sentinel error.
func ParseDisplayName(s string) (DisplayName, error) {
	trimmed := strings.TrimSpace(s)
	n := uniseg.GraphemeClusterCount(trimmed)
	if n < 1 {
		return "", ErrDisplayNameRequired
	}
	if n > DisplayNameMax {
		return "", ErrDisplayNameTooLong
	}
	return DisplayName(trimmed), nil
}
