package resolver

import (
	"testing"

	"backend/graph/model"
	"backend/internal/domain"
	"backend/internal/domain/service"
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

// TestSwipePerformanceModeMapper asserts that toSwipePerformanceModeModel maps
// every service.PerformanceMode to the matching generated wire enum, and that an
// out-of-range int falls back to DEFAULT. Because the mapper matches by named
// constant on both sides, this guards against a switch-body transposition (e.g.
// returning GOOD for ModeMastered) — a change that compiles and passes vet but
// only the DEFAULT arm is otherwise exercised end-to-end.
func TestSwipePerformanceModeMapper(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		mode service.PerformanceMode
		want model.SwipePerformanceMode
	}{
		{"Difficult", service.ModeDifficult, model.SwipePerformanceModeDifficult},
		{"Default", service.ModeDefault, model.SwipePerformanceModeDefault},
		{"Good", service.ModeGood, model.SwipePerformanceModeGood},
		{"Easy", service.ModeEasy, model.SwipePerformanceModeEasy},
		{"Mastered", service.ModeMastered, model.SwipePerformanceModeMastered},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := toSwipePerformanceModeModel(int(tc.mode)); got != tc.want {
				t.Fatalf("toSwipePerformanceModeModel(%d): got %q, want %q", tc.mode, got, tc.want)
			}
		})
	}

	t.Run("OutOfRangeFallsBackToDefault", func(t *testing.T) {
		t.Parallel()
		if got := toSwipePerformanceModeModel(99); got != model.SwipePerformanceModeDefault {
			t.Fatalf("toSwipePerformanceModeModel(99): got %q, want %q", got, model.SwipePerformanceModeDefault)
		}
	})
}
