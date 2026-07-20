package usecase

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"backend/internal/domain"
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

// TestTranslateCardErr pins the nil guard that keeps translateCardErr consistent
// with its siblings: without it a nil error falls to the default arm and is
// wrapped as an "unexpected domain error", turning a success into an INTERNAL
// failure. The sentinel and unexpected-error arms are covered alongside it so
// the guard cannot be added by short-circuiting the whole switch.
func TestTranslateCardErr(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		input     error
		wantField string
		wantMsg   string
		wantChain string
	}{
		{
			name:  "nil passes through",
			input: nil,
		},
		{
			name:      "front required sentinel",
			input:     domain.ErrCardFrontRequired,
			wantField: "front",
			wantMsg:   "front is required",
		},
		{
			name:      "back too-long sentinel",
			input:     domain.ErrCardBackTooLong,
			wantField: "back",
			wantMsg:   fmt.Sprintf("back must be at most %d characters", domain.CardTextMax),
		},
		{
			name:      "unexpected error wraps as internal",
			input:     errors.New("surprise"),
			wantChain: "usecase: translate card err: unexpected domain error",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := translateCardErr(tc.input)

			switch {
			case tc.input == nil:
				require.NoError(t, err, "a nil error must not be wrapped as an unexpected domain error")
			case tc.wantChain != "":
				assertInternalChain(t, err, tc.wantChain)
			default:
				require.Error(t, err)
				assertValidationError(t, err, tc.wantField, tc.wantMsg)
			}
		})
	}
}

// TestTranslateBioErr and TestTranslateDescriptionErr pin the thin wrappers over
// the shared translateTrinaryTextErr body: nil passes through, the too-long
// sentinel maps to a field-scoped ValidationError with the exact message, and any
// other error wraps into the field-specific internal chain.
func TestTranslateBioErr(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		input     error
		wantField string
		wantMsg   string
		wantChain string
	}{
		{
			name:  "nil passes through",
			input: nil,
		},
		{
			name:      "too-long sentinel",
			input:     domain.ErrBioTooLong,
			wantField: "bio",
			wantMsg:   fmt.Sprintf("bio must be at most %d characters", domain.BioMax),
		},
		{
			name:      "unexpected error wraps as internal",
			input:     errors.New("surprise"),
			wantChain: "usecase: translate bio error",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := translateBioErr(tc.input)

			switch {
			case tc.input == nil:
				require.NoError(t, err)
			case tc.wantChain != "":
				assertInternalChain(t, err, tc.wantChain)
			default:
				require.Error(t, err)
				assertValidationError(t, err, tc.wantField, tc.wantMsg)
			}
		})
	}
}

func TestTranslateDescriptionErr(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		input     error
		wantField string
		wantMsg   string
		wantChain string
	}{
		{
			name:  "nil passes through",
			input: nil,
		},
		{
			name:      "too-long sentinel",
			input:     domain.ErrDescriptionTooLong,
			wantField: "description",
			wantMsg:   fmt.Sprintf("description must be at most %d characters", domain.DescriptionMax),
		},
		{
			name:      "unexpected error wraps as internal",
			input:     errors.New("surprise"),
			wantChain: "usecase: translate description error",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := translateDescriptionErr(tc.input)

			switch {
			case tc.input == nil:
				require.NoError(t, err)
			case tc.wantChain != "":
				assertInternalChain(t, err, tc.wantChain)
			default:
				require.Error(t, err)
				assertValidationError(t, err, tc.wantField, tc.wantMsg)
			}
		})
	}
}

func TestTranslateDisplayNameErr(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		input     error
		wantField string // non-empty when a ValidationError is expected
		wantMsg   string // exact message when a ValidationError is expected
		wantChain string // non-empty when an internal-chain wrap is expected
	}{
		{
			name:  "nil passes through",
			input: nil,
		},
		{
			name:      "required sentinel",
			input:     domain.ErrDisplayNameRequired,
			wantField: "displayName",
			wantMsg:   "displayName is required",
		},
		{
			name:      "too-long sentinel",
			input:     domain.ErrDisplayNameTooLong,
			wantField: "displayName",
		},
		{
			name:      "reserved sentinel",
			input:     domain.ErrDisplayNameReserved,
			wantField: "displayName",
			wantMsg:   "displayName is reserved",
		},
		{
			name:      "unexpected error wraps as internal",
			input:     errors.New("surprise"),
			wantChain: "usecase: translate display name error",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := translateDisplayNameErr(tc.input)

			switch {
			case tc.input == nil:
				require.NoError(t, err)
			case tc.wantChain != "":
				assertInternalChain(t, err, tc.wantChain)
			default:
				require.Error(t, err)
				assertValidationError(t, err, tc.wantField, tc.wantMsg)
			}
		})
	}
}
