package domain

import (
	"database/sql/driver"
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

// Scan implements sql.Scanner so GORM can read a DisplayName column from the
// database without calling ParseDisplayName. The DB is trusted on reads;
// bound-checks are a write-side concern. For a nullable column the field is
// typed *DisplayName at the row struct, so Scan is only invoked for the
// non-nil case; nil source still maps to the empty string for safety.
func (d *DisplayName) Scan(src any) error {
	if src == nil {
		*d = ""
		return nil
	}
	switch v := src.(type) {
	case string:
		*d = DisplayName(v)
	case []byte:
		*d = DisplayName(v)
	default:
		return eris.Errorf("domain: display name: unsupported scan type %T", src)
	}
	return nil
}

// Value implements driver.Valuer so GORM writes the underlying string to the
// database column.
func (d DisplayName) Value() (driver.Value, error) { return string(d), nil }

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
