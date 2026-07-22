package domain

import (
	"errors"
	"strings"

	"github.com/rivo/uniseg"
	"github.com/rotisserie/eris"
)

const DisplayNameMax = 50

// Display name validation sentinels. The bound-free reasons use plain errors.New
// so errors.Is matches by identity rather than by eris's message equality; the
// bound-carrying reason stays on eris.Errorf so the exported DisplayNameMax and
// the message cannot drift apart.
var (
	ErrDisplayNameRequired = errors.New("user: display name is required")
	ErrDisplayNameTooLong  = eris.Errorf("user: display name exceeds %d characters", DisplayNameMax)
	ErrDisplayNameReserved = errors.New("user: display name is reserved")
)

// reservedDisplayNames is the set of lowercased display names that users may not
// claim, to prevent impersonation of the platform or staff. Exact normalized
// equality only — no substring matching.
var reservedDisplayNames = func() map[string]struct{} {
	entries := []string{
		"admin",
		"administrator",
		"root",
		"system",
		"support",
		"moderator",
		"mod",
		"staff",
		"official",
		"flamingo",
		"flamingo-armond",
		strings.ToLower(string(AdminRoleName)),
		strings.ToLower(string(GeneralRoleName)),
	}
	m := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		m[e] = struct{}{}
	}
	return m
}()

// DisplayName is the user-chosen profile display name, trimmed, 1..DisplayNameMax graphemes.
// The zero value (DisplayName("")) is invalid; use ParseDisplayName to construct.
type DisplayName string

// ParseDisplayName trims surrounding whitespace from s, counts grapheme clusters,
// checks against the reserved-name blocklist, and returns a validated DisplayName
// or a sentinel error.
func ParseDisplayName(s string) (DisplayName, error) {
	trimmed := strings.TrimSpace(s)
	n := uniseg.GraphemeClusterCount(trimmed)
	if n < 1 {
		return "", ErrDisplayNameRequired
	}
	if n > DisplayNameMax {
		return "", ErrDisplayNameTooLong
	}
	if _, reserved := reservedDisplayNames[strings.ToLower(trimmed)]; reserved {
		return "", ErrDisplayNameReserved
	}
	return DisplayName(trimmed), nil
}
