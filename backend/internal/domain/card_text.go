package domain

import (
	"strings"

	"github.com/rivo/uniseg"
)

// CardText is the trimmed, grapheme-bounded text used for both Card.Front and
// Card.Back. Caller passes its own field-specific sentinels — VO stays
// field-agnostic.
type CardText string

// String returns the underlying string value.
func (c CardText) String() string { return string(c) }

// ParseCardText trims s, counts grapheme clusters, and returns a validated
// CardText. It returns requiredErr when the trimmed input is empty and
// tooLongErr when it exceeds CardTextMax clusters.
func ParseCardText(s string, requiredErr, tooLongErr error) (CardText, error) {
	trimmed := strings.TrimSpace(s)
	n := uniseg.GraphemeClusterCount(trimmed)
	if n < 1 {
		return "", requiredErr
	}
	if n > CardTextMax {
		return "", tooLongErr
	}
	return CardText(trimmed), nil
}
