package domain

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseCardgroupName(t *testing.T) {
	t.Parallel()

	// A family emoji ZWJ sequence — one grapheme cluster, multiple runes.
	const zwjEmoji = "👨‍👩‍👧"

	cases := []struct {
		name        string
		input       string
		wantValue   CardgroupName
		sentinelErr error
	}{
		{
			name:      "happy short name",
			input:     "My Flashcards",
			wantValue: CardgroupName("My Flashcards"),
		},
		{
			name:      "happy surrounding whitespace trimmed",
			input:     "  My Flashcards  ",
			wantValue: CardgroupName("My Flashcards"),
		},
		{
			name:      "happy at exactly CardgroupNameMax graphemes",
			input:     strings.Repeat("a", CardgroupNameMax),
			wantValue: CardgroupName(strings.Repeat("a", CardgroupNameMax)),
		},
		{
			name:      "happy CJK characters",
			input:     "日本語フラッシュカード",
			wantValue: CardgroupName("日本語フラッシュカード"),
		},
		{
			name:        "empty string",
			input:       "",
			sentinelErr: ErrCardgroupNameRequired,
		},
		{
			name:        "whitespace only",
			input:       "   ",
			sentinelErr: ErrCardgroupNameRequired,
		},
		{
			name:        "CardgroupNameMax+1 ASCII chars",
			input:       strings.Repeat("a", CardgroupNameMax+1),
			sentinelErr: ErrCardgroupNameTooLong,
		},
		{
			name:        "CardgroupNameMax+1 ZWJ emojis",
			input:       strings.Repeat(zwjEmoji, CardgroupNameMax+1),
			sentinelErr: ErrCardgroupNameTooLong,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseCardgroupName(tc.input)
			if tc.sentinelErr == nil {
				require.NoError(t, err)
				require.Equal(t, tc.wantValue, got)
				return
			}
			require.Error(t, err)
			require.True(t, errors.Is(err, tc.sentinelErr), "got %v", err)
		})
	}
}

func TestCardgroupNameString(t *testing.T) {
	t.Parallel()

	got, err := ParseCardgroupName("My Cardgroup")
	require.NoError(t, err)
	require.Equal(t, "My Cardgroup", got.String())
}
