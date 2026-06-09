package domain

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseDisplayName(t *testing.T) {
	t.Parallel()

	const zwjEmoji = "👨‍👩‍👧‍👦"
	cases := []struct {
		name        string
		input       string
		wantValue   DisplayName
		sentinelErr error
	}{
		{
			name:      "happy short name",
			input:     "Alice",
			wantValue: DisplayName("Alice"),
		},
		{
			name:      "happy with surrounding whitespace",
			input:     "  Alice  ",
			wantValue: DisplayName("Alice"),
		},
		{
			name:      "happy at exactly 50 graphemes",
			input:     strings.Repeat(zwjEmoji, 50),
			wantValue: DisplayName(strings.Repeat(zwjEmoji, 50)),
		},
		{
			name:        "empty string",
			input:       "",
			sentinelErr: ErrDisplayNameRequired,
		},
		{
			name:        "whitespace only",
			input:       "   ",
			sentinelErr: ErrDisplayNameRequired,
		},
		{
			name:        "51 ASCII chars",
			input:       strings.Repeat("a", 51),
			sentinelErr: ErrDisplayNameTooLong,
		},
		{
			name:        "51 ZWJ emojis",
			input:       strings.Repeat(zwjEmoji, 51),
			sentinelErr: ErrDisplayNameTooLong,
		},

		// Reserved-name cases.
		{
			name:        "reserved exact lowercase admin",
			input:       "admin",
			sentinelErr: ErrDisplayNameReserved,
		},
		{
			name:        "reserved mixed-case Admin",
			input:       "Admin",
			sentinelErr: ErrDisplayNameReserved,
		},
		{
			name:        "reserved all-uppercase ADMIN",
			input:       "ADMIN",
			sentinelErr: ErrDisplayNameReserved,
		},
		{
			name:        "reserved with surrounding whitespace",
			input:       "  admin  ",
			sentinelErr: ErrDisplayNameReserved,
		},
		{
			name:        "reserved flamingo",
			input:       "flamingo",
			sentinelErr: ErrDisplayNameReserved,
		},
		{
			name:        "reserved system",
			input:       "system",
			sentinelErr: ErrDisplayNameReserved,
		},

		// Near-miss: must be accepted (substring match is NOT applied).
		{
			name:      "near-miss Adminah is accepted",
			input:     "Adminah",
			wantValue: DisplayName("Adminah"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseDisplayName(tc.input)
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
