package usecase

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestValidateRelayArgs covers every rejection branch and the happy paths.
// strPtr and intPtr are defined in card_test.go (same package).
func TestValidateRelayArgs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		first     *int
		last      *int
		after     *string
		before    *string
		wantField string // non-empty when a ValidationError is expected
	}{
		// --- Rejection branches ---

		{
			// after and before are mutually exclusive cursor directions.
			name:      "after_and_before_mutually_exclusive",
			after:     strPtr("cursor-a"),
			before:    strPtr("cursor-b"),
			wantField: "after",
		},
		{
			// first (forward count) cannot pair with before (backward cursor).
			name:      "first_with_before",
			first:     intPtr(5),
			before:    strPtr("cursor-b"),
			wantField: "before",
		},
		{
			// last (backward count) cannot pair with after (forward cursor).
			name:      "last_with_after",
			last:      intPtr(5),
			after:     strPtr("cursor-a"),
			wantField: "after",
		},
		{
			// before without any companion count is ambiguous.
			name:      "before_without_count",
			before:    strPtr("cursor-b"),
			wantField: "before",
		},
		{
			// before with first=0 is also without a usable count.
			name:      "before_with_first_zero",
			first:     intPtr(0),
			before:    strPtr("cursor-b"),
			wantField: "before",
		},
		{
			// after without any companion count is ambiguous.
			name:      "after_without_count",
			after:     strPtr("cursor-a"),
			wantField: "after",
		},
		{
			// after with last=0 is also without a usable count.
			name:      "after_with_last_zero",
			last:      intPtr(0),
			after:     strPtr("cursor-a"),
			wantField: "after",
		},

		// --- Happy paths (no error expected) ---

		{
			// Forward page: first + after.
			name:  "forward_first_and_after",
			first: intPtr(10),
			after: strPtr("cursor-a"),
		},
		{
			// Backward page: last + before.
			name:   "backward_last_and_before",
			last:   intPtr(10),
			before: strPtr("cursor-b"),
		},
		{
			// Initial forward page: first only, no cursor.
			name:  "first_only",
			first: intPtr(10),
		},
		{
			// Initial backward page: last only, no cursor.
			name: "last_only",
			last: intPtr(10),
		},
		{
			// No arguments at all — caller uses its own default.
			name: "all_nil",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validateRelayArgs(tc.first, tc.last, tc.after, tc.before)

			if tc.wantField == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assertValidationError(t, err, tc.wantField, "")
			}
		})
	}
}
