package domain

import (
	"database/sql/driver"
	"strings"

	"github.com/rivo/uniseg"
	"github.com/rotisserie/eris"
)

// CardText is the trimmed, grapheme-bounded text used for both Card.Front and
// Card.Back. Caller passes its own field-specific sentinels — VO stays
// field-agnostic (1..CardTextMax graphemes).
// The zero value (CardText("")) is invalid; use ParseCardText to construct.
type CardText string

// String returns the underlying string value.
func (c CardText) String() string { return string(c) }

// Scan implements sql.Scanner so GORM can read a CardText column from the
// database without calling ParseCardText. The DB is trusted on reads;
// bound-checks are a write-side concern.
func (c *CardText) Scan(src any) error {
	if src == nil {
		*c = ""
		return nil
	}
	switch v := src.(type) {
	case string:
		*c = CardText(v)
	case []byte:
		*c = CardText(v)
	default:
		return eris.Errorf("domain: card text: unsupported scan type %T", src)
	}
	return nil
}

// Value implements driver.Valuer so GORM writes the underlying string to the
// database column.
func (c CardText) Value() (driver.Value, error) { return string(c), nil }

// ParseCardText trims s, counts grapheme clusters, and returns a validated
// CardText. It returns requiredErr when the trimmed input is empty and
// tooLongErr when it exceeds CardTextMax clusters.
func ParseCardText(s string, requiredErr, tooLongErr error) (CardText, error) {
	if requiredErr == nil || tooLongErr == nil {
		panic("domain: ParseCardText requires non-nil requiredErr and tooLongErr sentinels")
	}
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
