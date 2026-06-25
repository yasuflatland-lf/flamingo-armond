package domain

import (
	"strings"

	"github.com/rivo/uniseg"
	"github.com/rotisserie/eris"
)

const DescriptionMax = 500

// ErrDescriptionTooLong is returned when the trimmed description exceeds
// DescriptionMax graphemes.
var ErrDescriptionTooLong = eris.Errorf("master cardgroup: description exceeds %d characters", DescriptionMax)

// Description is the optional master-cardgroup description, supporting a trinary:
// nil (no change), pointer-to-"" (explicit clear), or pointer-to-non-empty (set).
// It mirrors Bio: an optional, nullable, grapheme-bounded text field on an
// aggregate. The zero value Description{} is the no-value case.
type Description struct {
	value *string
}

// ParseDescription validates and trims s, preserving the trinary contract:
// nil means "no change"; a pointer to "" means "explicit clear"; a pointer to a
// non-empty string means "set". Surrounding whitespace is trimmed; whitespace-only
// inputs collapse to the explicit-clear case (Ptr() returns a pointer to "").
// Returns ErrDescriptionTooLong if the trimmed value exceeds DescriptionMax graphemes.
func ParseDescription(s *string) (Description, error) {
	if s == nil {
		return Description{}, nil
	}
	trimmed := strings.TrimSpace(*s)
	if uniseg.GraphemeClusterCount(trimmed) > DescriptionMax {
		return Description{}, ErrDescriptionTooLong
	}
	return Description{value: &trimmed}, nil
}

// DescriptionFromPtr maps a nullable text column to a Description: nil → Description{}
// (IsSet()=false, NULL column); a non-nil pointer — including pointer-to-empty —
// maps to a set Description whose Ptr() returns a defensive copy with the same
// string value. Used by repository readers to bridge a nullable text column into
// the typed domain field.
func DescriptionFromPtr(p *string) Description {
	if p == nil {
		return Description{}
	}
	s := *p
	return Description{value: &s}
}

// Ptr returns a fresh copy of the internal pointer:
//   - nil — the Description holds no value (constructed via ParseDescription(nil) or DescriptionFromPtr(nil));
//   - non-nil pointer to empty string — the Description holds an empty value;
//   - non-nil pointer to non-empty string — the Description holds a populated value.
//
// Patch-context callers map nil → "no change", &"" → "explicit clear", &"x" → "set".
// Read-context callers map nil → NULL column, &"" → empty stored, &"x" → populated stored.
func (d Description) Ptr() *string {
	if d.value == nil {
		return nil
	}
	s := *d.value
	return &s
}

// IsSet reports whether the Description carries a value (its internal pointer is
// non-nil). Returns false for ParseDescription(nil) (patch-context: no change) and
// for DescriptionFromPtr(nil) (read-context: NULL column / zero value Description{}).
// Returns true for any other construction, including an explicit empty string.
func (d Description) IsSet() bool { return d.value != nil }
