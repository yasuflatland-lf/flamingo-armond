package resolver

import (
	"testing"

	"backend/internal/domain"
)

// TestLearnDisplayModeMapper_RoundTrip asserts that converting a domain value to
// the GraphQL model and back yields the original value for every valid enum member.
func TestLearnDisplayModeMapper_RoundTrip(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		domainMode domain.LearnDisplayMode
	}{
		{"FlipToReveal", domain.LearnDisplayFlipToReveal},
		{"AlwaysVisible", domain.LearnDisplayAlwaysVisible},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := fromLearnDisplayModeModel(toLearnDisplayModeModel(tc.domainMode))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.domainMode {
				t.Fatalf("round-trip: got %q, want %q", got, tc.domainMode)
			}
		})
	}
}
