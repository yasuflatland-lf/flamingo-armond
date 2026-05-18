package domain

import (
	"regexp"
	"strings"

	"github.com/rotisserie/eris"
)

// roleNameMax is the maximum byte length of a normalized role name. Because the
// pattern restricts input to ASCII, byte length equals grapheme count.
const roleNameMax = 30

// roleNamePattern allows lowercase letters, digits, underscores, and hyphens.
var roleNamePattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

var (
	ErrRoleNameRequired = eris.New("role: name is required")
	ErrRoleNameTooLong  = eris.Errorf("role: name exceeds %d characters", roleNameMax)
	ErrRoleNameInvalid  = eris.New("role: name must contain only lowercase letters, digits, '_', or '-'")
)

// RoleName is the canonical, lowercase, slug-like identifier for an application
// role. It is the normalized form of the user-supplied name string, validated
// against a strict character set and length limit.
type RoleName string

// ParseRoleName normalizes s (trim whitespace, lowercase) and validates it.
// It returns ErrRoleNameRequired, ErrRoleNameTooLong, or ErrRoleNameInvalid on
// failure, or the validated RoleName on success.
func ParseRoleName(s string) (RoleName, error) {
	normalized := strings.ToLower(strings.TrimSpace(s))
	if normalized == "" {
		return "", ErrRoleNameRequired
	}
	if len(normalized) > roleNameMax {
		return "", ErrRoleNameTooLong
	}
	if !roleNamePattern.MatchString(normalized) {
		return "", ErrRoleNameInvalid
	}
	return RoleName(normalized), nil
}
