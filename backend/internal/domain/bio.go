package domain

import "github.com/rotisserie/eris"

const BioMax = 500

// ErrBioTooLong is returned when the trimmed bio exceeds BioMax graphemes.
var ErrBioTooLong = eris.Errorf("user: bio exceeds %d characters", BioMax)

// Bio is the user profile bio: a trinary text value object bounded at BioMax
// graphemes. It supports nil (no change), pointer-to-"" (explicit clear), or
// pointer-to-non-empty (set). It embeds the shared trinaryText so the trim +
// grapheme-cap + trinary rule and the Ptr()/IsSet() accessors are single-sourced
// with Description; see trinary_text.go. The zero value Bio{} is the no-value case
// (IsSet()=false).
type Bio struct {
	trinaryText
}

// ParseBio validates and trims s, preserving the trinary contract:
// nil means "no change"; a pointer to "" means "explicit clear"; a pointer to
// a non-empty string means "set". Surrounding whitespace is trimmed;
// whitespace-only inputs collapse to the explicit-clear case (Ptr() returns
// a pointer to ""). Returns ErrBioTooLong if the trimmed value exceeds BioMax graphemes.
func ParseBio(s *string) (Bio, error) {
	t, err := parseTrinaryText(s, BioMax, ErrBioTooLong)
	return Bio{t}, err
}

// BioFromPtr maps a nullable text column to a Bio: nil → Bio{} (IsSet()=false,
// NULL column); a non-nil pointer — including pointer-to-empty — maps to a set
// Bio whose Ptr() returns a defensive copy with the same string value.
// Used by repository readers to bridge a nullable text column into the typed
// domain field.
func BioFromPtr(p *string) Bio {
	return Bio{trinaryTextFromPtr(p)}
}
