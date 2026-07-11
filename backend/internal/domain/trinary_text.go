package domain

import (
	"strings"

	"github.com/rivo/uniseg"
)

// trinaryText is the shared value object behind the optional, trimmed,
// grapheme-bounded text fields Bio and Description. It encodes a trinary via a
// private *string:
//   - nil          — no value ("no change" in a patch context; NULL column in a read context)
//   - &""          — an explicit empty value ("explicit clear" / empty stored)
//   - &"non-empty" — a populated value ("set" / populated stored)
//
// The VO is field-agnostic: parseTrinaryText takes the caller-supplied cap and
// too-long sentinel, mirroring ParseCardText's caller-supplied sentinels. Bio and
// Description embed it so the trim + grapheme-cap + trinary rule is single-sourced.
type trinaryText struct {
	value *string
}

// parseTrinaryText validates and trims s, preserving the trinary contract:
// nil means "no change"; a pointer to "" means "explicit clear"; a pointer to a
// non-empty string means "set". Surrounding whitespace is trimmed; whitespace-only
// inputs collapse to the explicit-clear case (Ptr() returns a pointer to "").
// Returns tooLongErr if the trimmed value exceeds max graphemes. It panics on a
// nil tooLongErr sentinel, mirroring ParseCardText's construction-boundary guard:
// a nil sentinel would return a nil error on a too-long input, which the caller
// would misread as "valid".
func parseTrinaryText(s *string, max int, tooLongErr error) (trinaryText, error) {
	if tooLongErr == nil {
		panic("domain: parseTrinaryText requires a non-nil tooLongErr sentinel")
	}
	if s == nil {
		return trinaryText{}, nil
	}
	trimmed := strings.TrimSpace(*s)
	if uniseg.GraphemeClusterCount(trimmed) > max {
		return trinaryText{}, tooLongErr
	}
	return trinaryText{value: &trimmed}, nil
}

// trinaryTextFromPtr maps a nullable text column to a trinaryText: nil → the
// no-value case (IsSet()=false, NULL column); a non-nil pointer — including
// pointer-to-empty — maps to a set value whose Ptr() returns a defensive copy with
// the same string value. Used by repository readers to bridge a nullable text
// column into the typed domain field.
func trinaryTextFromPtr(p *string) trinaryText {
	if p == nil {
		return trinaryText{}
	}
	s := *p
	return trinaryText{value: &s}
}

// Ptr returns a fresh copy of the internal pointer:
//   - nil — no value (constructed from a nil input);
//   - non-nil pointer to empty string — an empty value;
//   - non-nil pointer to non-empty string — a populated value.
//
// Patch-context callers map nil → "no change", &"" → "explicit clear", &"x" → "set".
// Read-context callers map nil → NULL column, &"" → empty stored, &"x" → populated stored.
func (t trinaryText) Ptr() *string {
	if t.value == nil {
		return nil
	}
	s := *t.value
	return &s
}

// IsSet reports whether the value carries a payload (its internal pointer is
// non-nil). Returns false for a nil-input parse (patch-context: no change) and for
// a nil-pointer read (read-context: NULL column / zero value). Returns true for any
// other construction, including an explicit empty string.
func (t trinaryText) IsSet() bool { return t.value != nil }
