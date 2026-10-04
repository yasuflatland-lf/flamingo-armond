package domain

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestParseDisplayName pins the reserved-name blocklist: exact match after
// trim and case folding, never a substring match. Trim and the grapheme cap are
// covered by TestRequiredTextVO_Property_TrimAndGraphemeCap.
func TestParseDisplayName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		input       string
		wantValue   DisplayName
		sentinelErr error
	}{
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
