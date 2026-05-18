package domain

import (
	"database/sql/driver"
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

// TestDisplayName_ScanValueRoundTrip exercises the sql.Scanner and
// driver.Valuer methods so a nullable display_name column round-trips through
// GORM without losing fidelity. nil source maps to "" (the empty newtype
// value); string and []byte sources both populate the field; Value emits the
// underlying string.
func TestDisplayName_ScanValueRoundTrip(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		src  any
		want DisplayName
	}{
		{name: "nil source maps to empty newtype", src: nil, want: ""},
		{name: "string source", src: "Alice", want: DisplayName("Alice")},
		{name: "[]byte source", src: []byte("Bob"), want: DisplayName("Bob")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var d DisplayName
			require.NoError(t, d.Scan(tc.src))
			require.Equal(t, tc.want, d)

			v, err := d.Value()
			require.NoError(t, err)
			require.Equal(t, driver.Value(string(tc.want)), v)
		})
	}
}

// TestDisplayName_ScanUnsupportedType verifies that an unexpected source type
// surfaces as an eris-wrapped error rather than panicking or silently
// truncating.
func TestDisplayName_ScanUnsupportedType(t *testing.T) {
	t.Parallel()

	var d DisplayName
	err := d.Scan(123)
	require.Error(t, err)
}
