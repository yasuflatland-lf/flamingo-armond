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
// whitespace-only inputs collapse to the explicit-clear case (Ptr() returns
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

// BioFromPtr maps a nullable text column to a Bio: nil → Bio{} (IsSet()=false,
// NULL column); a non-nil pointer — including pointer-to-empty — maps to a set
// Bio whose Ptr() returns a defensive copy with the same string value.
// Used by repository readers to bridge a nullable text column into the typed
// domain field.
func BioFromPtr(p *string) Bio {
	if p == nil {
		return Bio{}
	}
	s := *p
	return Bio{value: &s}
}

// Ptr returns a fresh copy of the internal pointer:
//   - nil — the Bio holds no value (constructed via ParseBio(nil) or BioFromPtr(nil));
//   - non-nil pointer to empty string — the Bio holds an empty value;
//   - non-nil pointer to non-empty string — the Bio holds a populated value.
//
// Patch-context callers map nil → "no change", &"" → "explicit clear", &"x" → "set".
// Read-context callers map nil → NULL column, &"" → empty stored, &"x" → populated stored.
func (b Bio) Ptr() *string {
	if b.value == nil {
		return nil
	}
	s := *b.value
	return &s
}

// IsSet reports whether the Bio carries a value (its internal pointer is non-nil).
// Returns false for ParseBio(nil) (patch-context: no change) and for BioFromPtr(nil)
// (read-context: NULL column / zero value Bio{}). Returns true for any other
// construction, including an explicit empty string.
func (b Bio) IsSet() bool { return b.value != nil }
