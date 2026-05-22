package usecase

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateRelayArgs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		first     *int
		last      *int
		after     *string
		before    *string
		wantField string // non-empty when a ValidationError is expected
		wantMsg   string // exact message string produced by the failing branch
	}{
		// --- Rejection branches ---

		{
			name:      "after_and_before_mutually_exclusive",
			after:     strPtr("cursor-a"),
			before:    strPtr("cursor-b"),
			wantField: "after",
			wantMsg:   "after and before are mutually exclusive",
		},
		{
			name:      "first_with_before",
			first:     intPtr(5),
			before:    strPtr("cursor-b"),
			wantField: "before",
			wantMsg:   "before requires last, not first",
		},
		{
			name:      "last_with_after",
			last:      intPtr(5),
			after:     strPtr("cursor-a"),
			wantField: "after",
			wantMsg:   "after requires first, not last",
		},
		{
			name:      "before_without_count",
			before:    strPtr("cursor-b"),
			wantField: "before",
			wantMsg:   "before requires last",
		},
		{
			name:      "before_with_first_zero",
			first:     intPtr(0),
			before:    strPtr("cursor-b"),
			wantField: "before",
			wantMsg:   "before requires last",
		},
		{
			name:      "after_without_count",
			after:     strPtr("cursor-a"),
			wantField: "after",
			wantMsg:   "after requires first",
		},
		{
			name:      "after_with_last_zero",
			last:      intPtr(0),
			after:     strPtr("cursor-a"),
			wantField: "after",
			wantMsg:   "after requires first",
		},

		// --- Happy paths (no error expected) ---

		{
			name:  "forward_first_and_after",
			first: intPtr(10),
			after: strPtr("cursor-a"),
		},
		{
			name:   "backward_last_and_before",
			last:   intPtr(10),
			before: strPtr("cursor-b"),
		},
		{
			name:  "first_only",
			first: intPtr(10),
		},
		{
			name: "last_only",
			last: intPtr(10),
		},
		{
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
				assertValidationError(t, err, tc.wantField, tc.wantMsg)
			}
		})
	}
}
