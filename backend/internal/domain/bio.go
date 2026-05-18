package domain

import (
	"strings"

	"github.com/rivo/uniseg"
	"github.com/rotisserie/eris"
)

const BioMax = 500

// ErrBioTooLong is returned when the trimmed bio exceeds BioMax graphemes.
var ErrBioTooLong = eris.Errorf("user: bio exceeds %d characters", BioMax)

// Bio is the user profile bio, supporting a trinary: nil (no change),
// pointer-to-"" (explicit clear), or pointer-to-non-empty (set).
type Bio struct {
	value *string
}

// ParseBio validates and trims s, preserving the trinary contract:
// nil means "no change"; a pointer to "" means "explicit clear"; a pointer to
// a non-empty string means "set". Surrounding whitespace is trimmed;
// whitespace-only inputs collapse to the explicit-clear case (Value() returns
// a pointer to ""). Returns ErrBioTooLong if the trimmed value exceeds BioMax graphemes.
func ParseBio(s *string) (Bio, error) {
	if s == nil {
		return Bio{}, nil
	}
	trimmed := strings.TrimSpace(*s)
	if uniseg.GraphemeClusterCount(trimmed) > BioMax {
		return Bio{}, ErrBioTooLong
	}
	return Bio{value: &trimmed}, nil
}

// Value returns a copy of the persistence-ready pointer: nil for "no change",
// pointer-to-"" for explicit clear, pointer-to-non-empty for set.
func (b Bio) Value() *string {
	if b.value == nil {
		return nil
	}
	s := *b.value
	return &s
}

// IsSet reports whether the caller supplied a value (including an explicit clear).
// Returns false only when the Bio was constructed from a nil input.
func (b Bio) IsSet() bool { return b.value != nil }
