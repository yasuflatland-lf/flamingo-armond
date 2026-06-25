package domain

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDescriptionFromPtr verifies the helper that bridges a *string repository
// read into the trinary VO. nil → Description{} (IsSet=false); non-nil pointer
// copies the underlying string and exposes it via Ptr() in defensive-copy fashion.
func TestDescriptionFromPtr(t *testing.T) {
	t.Parallel()

	t.Run("nil pointer maps to unset Description", func(t *testing.T) {
		t.Parallel()
		d := DescriptionFromPtr(nil)
		require.False(t, d.IsSet())
		require.Nil(t, d.Ptr())
	})

	t.Run("pointer to non-empty string maps to set Description", func(t *testing.T) {
		t.Parallel()
		s := "an intro deck"
		d := DescriptionFromPtr(&s)
		require.True(t, d.IsSet())
		require.NotNil(t, d.Ptr())
		require.Equal(t, "an intro deck", *d.Ptr())
	})

	t.Run("pointer to empty string maps to set Description holding empty string", func(t *testing.T) {
		t.Parallel()
		s := ""
		d := DescriptionFromPtr(&s)
		require.True(t, d.IsSet())
		require.NotNil(t, d.Ptr())
		require.Equal(t, "", *d.Ptr())
	})

	t.Run("Ptr returns a fresh pointer on each call", func(t *testing.T) {
		t.Parallel()
		s := "original"
		d := DescriptionFromPtr(&s)
		p1, p2 := d.Ptr(), d.Ptr()
		require.NotSame(t, p1, p2, "each Ptr() call must allocate a fresh pointer")
		require.Equal(t, *p1, *p2, "but the values must be equal")
	})
}

func TestParseDescription(t *testing.T) {
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
			input:     strPtr("A short deck description"),
			wantIsSet: true,
			wantValue: strPtr("A short deck description"),
		},
		{
			name:      "surrounding whitespace trimmed",
			input:     strPtr("  hello  "),
			wantIsSet: true,
			wantValue: strPtr("hello"),
		},
		{
			name:      "exactly DescriptionMax graphemes — ok",
			input:     strPtr(strings.Repeat("a", DescriptionMax)),
			wantIsSet: true,
			wantValue: strPtr(strings.Repeat("a", DescriptionMax)),
		},
		{
			name:        "DescriptionMax+1 ASCII chars — too long",
			input:       strPtr(strings.Repeat("a", DescriptionMax+1)),
			sentinelErr: ErrDescriptionTooLong,
		},
		{
			name:        "DescriptionMax+1 ZWJ emojis — grapheme count, not bytes",
			input:       strPtr(strings.Repeat(zwjEmoji, DescriptionMax+1)),
			sentinelErr: ErrDescriptionTooLong,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseDescription(tc.input)
			if tc.sentinelErr != nil {
				require.Error(t, err)
				require.True(t, errors.Is(err, tc.sentinelErr), "got %v", err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantIsSet, got.IsSet())
			require.Equal(t, tc.wantValue, got.Ptr())
		})
	}
}
