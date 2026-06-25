package domain

import (
	"strings"

	"github.com/rivo/uniseg"
	"github.com/rotisserie/eris"
)

// ErrTextTooLong is returned by ParseBoundedText when the trimmed input exceeds
// the caller-supplied maximum grapheme-cluster length. The field identity and
// the concrete bound live at the usecase call site (the VO is field-agnostic),
// so a single shared sentinel suffices.
var ErrTextTooLong = eris.New("domain: bounded text exceeds maximum length")

// BoundedText is trimmed free-form text validated against a caller-supplied
// maximum grapheme-cluster length. Unlike CardText it has no minimum: every
// field that uses it is optional. The zero value is the empty string; use
// ParseBoundedText to construct a validated value.
type BoundedText string

// String returns the underlying string value.
func (b BoundedText) String() string { return string(b) }

// ParseBoundedText trims s and returns a BoundedText when its grapheme-cluster
// count is at most max, otherwise ErrTextTooLong. max must be positive.
func ParseBoundedText(s string, max int) (BoundedText, error) {
	if max < 1 {
		panic("domain: ParseBoundedText requires a positive max")
	}
	trimmed := strings.TrimSpace(s)
	if uniseg.GraphemeClusterCount(trimmed) > max {
		return "", ErrTextTooLong
	}
	return BoundedText(trimmed), nil
}
