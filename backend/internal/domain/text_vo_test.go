package domain

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRequiredTextVO_ZWJGraphemeCapPins pins, for every required text VO, that
// the cap counts grapheme clusters: max ZWJ family emoji are accepted and max+1
// rejected. The general law is TestRequiredTextVO_Property_TrimAndGraphemeCap.
func TestRequiredTextVO_ZWJGraphemeCapPins(t *testing.T) {
	t.Parallel()

	const zwjEmoji = "\U0001F468\u200d\U0001F469\u200d\U0001F467\u200d\U0001F466"
	errReq, errLong := errors.New("required"), errors.New("too long")
	cases := []struct {
		name    string
		max     int
		tooLong error
		parse   func(string) error
	}{
		{"CardText", CardTextMax, errLong, func(s string) error { _, err := ParseCardText(s, errReq, errLong); return err }},
		{"CardgroupName", CardgroupNameMax, ErrCardgroupNameTooLong, func(s string) error { _, err := ParseCardgroupName(s); return err }},
		{"DisplayName", DisplayNameMax, ErrDisplayNameTooLong, func(s string) error { _, err := ParseDisplayName(s); return err }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.NoError(t, tc.parse(strings.Repeat(zwjEmoji, tc.max)))
			require.ErrorIs(t, tc.parse(strings.Repeat(zwjEmoji, tc.max+1)), tc.tooLong)
		})
	}
}

// TestTrinaryText_Pins keeps the two Bio/Description contracts a reader looks
// for by name: whitespace-only input is an explicit clear (set, empty), and the
// cap counts graphemes, so 501 ZWJ family emoji are rejected although each is
// many bytes. The general law is TestTrinaryText_Property_BioAndDescription.
func TestTrinaryText_Pins(t *testing.T) {
	t.Parallel()

	const zwjEmoji = "\U0001F468\u200d\U0001F469\u200d\U0001F467\u200d\U0001F466"
	blank, overCap := "   ", strings.Repeat(zwjEmoji, BioMax+1)

	bio, err := ParseBio(&blank)
	require.NoError(t, err)
	require.NotNil(t, bio.Ptr())
	require.Equal(t, "", *bio.Ptr())
	desc, err := ParseDescription(&blank)
	require.NoError(t, err)
	require.NotNil(t, desc.Ptr())
	require.Equal(t, "", *desc.Ptr())

	_, err = ParseBio(&overCap)
	require.ErrorIs(t, err, ErrBioTooLong)
	_, err = ParseDescription(&overCap)
	require.ErrorIs(t, err, ErrDescriptionTooLong)
}
