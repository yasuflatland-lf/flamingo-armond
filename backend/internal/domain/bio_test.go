package domain

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseBio(t *testing.T) {
	t.Parallel()

	const zwjEmoji = "👨‍👩‍👧‍👦"

	cases := []struct {
		name        string
		input       *string
		wantIsSet   bool
		wantValue   *string
		sentinelErr error
	}{
		{
			name:      "nil input — no change",
			input:     nil,
			wantIsSet: false,
			wantValue: nil,
		},
		{
			name:      "empty pointer — explicit clear",
			input:     strPtr(""),
			wantIsSet: true,
			wantValue: strPtr(""),
		},
		{
			name:      "whitespace only — trimmed to explicit clear",
			input:     strPtr("   "),
			wantIsSet: true,
			wantValue: strPtr(""),
		},
		{
			name:      "normal string",
			input:     strPtr("Hello world"),
			wantIsSet: true,
			wantValue: strPtr("Hello world"),
		},
		{
			name:      "surrounding whitespace trimmed",
			input:     strPtr("  hello  "),
			wantIsSet: true,
			wantValue: strPtr("hello"),
		},
		{
			name:      "exactly 500 graphemes — ok",
			input:     strPtr(strings.Repeat("a", 500)),
			wantIsSet: true,
			wantValue: strPtr(strings.Repeat("a", 500)),
		},
		{
			name:        "501 ASCII chars — too long",
			input:       strPtr(strings.Repeat("a", 501)),
			sentinelErr: ErrBioTooLong,
		},
		{
			name:        "501 ZWJ emojis — grapheme count, not bytes",
			input:       strPtr(strings.Repeat(zwjEmoji, 501)),
			sentinelErr: ErrBioTooLong,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseBio(tc.input)
			if tc.sentinelErr != nil {
				require.Error(t, err)
				require.True(t, errors.Is(err, tc.sentinelErr), "got %v", err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantIsSet, got.IsSet())
			if tc.wantValue == nil {
				require.Nil(t, got.Value())
			} else {
				require.NotNil(t, got.Value())
				require.Equal(t, *tc.wantValue, *got.Value())
			}
		})
	}
}

func strPtr(s string) *string { return &s }
