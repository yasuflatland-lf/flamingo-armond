package domain

import (
	"strings"

	"github.com/rivo/uniseg"
)

// CardgroupName is the user-supplied display name for a cardgroup, trimmed,
// 1..CardgroupNameMax grapheme clusters.
// The zero value (CardgroupName("")) is invalid; use ParseCardgroupName to construct.
type CardgroupName string

// ParseCardgroupName trims surrounding whitespace from s, counts grapheme
// clusters, and returns a validated CardgroupName or a sentinel error.
// Returns ErrCardgroupNameRequired when the trimmed string is empty, or
// ErrCardgroupNameTooLong when it exceeds CardgroupNameMax grapheme clusters.
func ParseCardgroupName(s string) (CardgroupName, error) {
	trimmed := strings.TrimSpace(s)
	n := uniseg.GraphemeClusterCount(trimmed)
	if n < 1 {
		return "", ErrCardgroupNameRequired
	}
	if n > CardgroupNameMax {
		return "", ErrCardgroupNameTooLong
	}
	return CardgroupName(trimmed), nil
}

// String returns the underlying string value of the CardgroupName.
func (c CardgroupName) String() string {
	return string(c)
}
