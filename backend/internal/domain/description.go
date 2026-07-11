package domain

import "github.com/rotisserie/eris"

const DescriptionMax = 500

// ErrDescriptionTooLong is returned when the trimmed description exceeds
// DescriptionMax graphemes.
var ErrDescriptionTooLong = eris.Errorf("master cardgroup: description exceeds %d characters", DescriptionMax)

// Description is the optional master-cardgroup description: a trinary text value
// object bounded at DescriptionMax graphemes. It supports nil (no change),
// pointer-to-"" (explicit clear), or pointer-to-non-empty (set). It embeds the
// shared trinaryText so the trim + grapheme-cap + trinary rule and the
// Ptr()/IsSet() accessors are single-sourced with Bio; see trinary_text.go. The
// zero value Description{} is the no-value case (IsSet()=false).
type Description struct {
	trinaryText
}

// ParseDescription validates and trims s, preserving the trinary contract:
// nil means "no change"; a pointer to "" means "explicit clear"; a pointer to a
// non-empty string means "set". Surrounding whitespace is trimmed; whitespace-only
// inputs collapse to the explicit-clear case (Ptr() returns a pointer to "").
// Returns ErrDescriptionTooLong if the trimmed value exceeds DescriptionMax graphemes.
func ParseDescription(s *string) (Description, error) {
	t, err := parseTrinaryText(s, DescriptionMax, ErrDescriptionTooLong)
	return Description{t}, err
}

// DescriptionFromPtr maps a nullable text column to a Description: nil → Description{}
// (IsSet()=false, NULL column); a non-nil pointer — including pointer-to-empty —
// maps to a set Description whose Ptr() returns a defensive copy with the same
// string value. Used by repository readers to bridge a nullable text column into
// the typed domain field.
func DescriptionFromPtr(p *string) Description {
	return Description{trinaryTextFromPtr(p)}
}
