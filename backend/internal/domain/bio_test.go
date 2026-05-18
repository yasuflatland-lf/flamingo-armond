package domain

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestBio_ScanValuerRoundTrip exercises sql.Scanner and driver.Valuer so a
// nullable bio column round-trips through GORM with the trinary contract
// intact: null DB → Bio{} (IsSet=false), text DB → set Bio (IsSet=true),
// including the explicit-clear empty-string case.
func TestBio_ScanValuerRoundTrip(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		src       any
		wantIsSet bool
		wantPtr   *string
	}{
		{name: "nil source → no-change Bio", src: nil, wantIsSet: false, wantPtr: nil},
		{name: "string source set", src: "hello", wantIsSet: true, wantPtr: strPtr("hello")},
		{name: "empty string source → explicit clear", src: "", wantIsSet: true, wantPtr: strPtr("")},
		{name: "[]byte source set", src: []byte("from bytes"), wantIsSet: true, wantPtr: strPtr("from bytes")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var b Bio
			require.NoError(t, b.Scan(tc.src))
			require.Equal(t, tc.wantIsSet, b.IsSet())
			if tc.wantPtr == nil {
				require.Nil(t, b.Ptr())
			} else {
				require.NotNil(t, b.Ptr())
				require.Equal(t, *tc.wantPtr, *b.Ptr())
			}

			v, err := b.Value()
			require.NoError(t, err)
			if tc.wantPtr == nil {
				require.Nil(t, v)
			} else {
				require.Equal(t, *tc.wantPtr, v)
			}
		})
	}
}

// TestBio_ScanUnsupportedType verifies that an unexpected source type
// surfaces as an eris-wrapped error rather than panicking.
func TestBio_ScanUnsupportedType(t *testing.T) {
	t.Parallel()

	var b Bio
	err := b.Scan(123)
	require.Error(t, err)
}

// TestBioFromPtr verifies the helper that bridges a *string repository read
// into the trinary VO. nil → Bio{} (IsSet=false); non-nil pointer copies the
// underlying string and exposes it via Ptr() in defensive-copy fashion.
func TestBioFromPtr(t *testing.T) {
	t.Parallel()

	t.Run("nil pointer maps to no-change Bio", func(t *testing.T) {
		t.Parallel()
		b := BioFromPtr(nil)
		require.False(t, b.IsSet())
		require.Nil(t, b.Ptr())
	})

	t.Run("pointer to non-empty string maps to set Bio", func(t *testing.T) {
		t.Parallel()
		s := "hello"
		b := BioFromPtr(&s)
		require.True(t, b.IsSet())
		require.NotNil(t, b.Ptr())
		require.Equal(t, "hello", *b.Ptr())
	})

	t.Run("pointer to empty string maps to explicit-clear Bio", func(t *testing.T) {
		t.Parallel()
		s := ""
		b := BioFromPtr(&s)
		require.True(t, b.IsSet())
		require.NotNil(t, b.Ptr())
		require.Equal(t, "", *b.Ptr())
	})

	t.Run("input pointer mutation does not affect VO", func(t *testing.T) {
		t.Parallel()
		s := "original"
		b := BioFromPtr(&s)
		s = "mutated"
		require.Equal(t, "original", *b.Ptr())
	})
}

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
				require.Nil(t, got.Ptr())
			} else {
				require.NotNil(t, got.Ptr())
				require.Equal(t, *tc.wantValue, *got.Ptr())
			}
		})
	}
}

func strPtr(s string) *string { return &s }
