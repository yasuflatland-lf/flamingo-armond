package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestClassifyMastery_Boundary pins the three-tier mastery classifier at the
// stability boundary (21d, inclusive) across every FSRSPhase. The threshold is
// passed explicitly so the boundary is exercised independently of the
// MatureStabilityDays const.
func TestClassifyMastery_Boundary(t *testing.T) {
	t.Parallel()

	const threshold = 21.0

	cases := []struct {
		name      string
		phase     FSRSPhase
		stability float64
		want      MasteryTier
	}{
		// Non-Review phases are always InProgress regardless of stability.
		{"new below threshold", FSRSPhaseNew, 20.9, TierInProgress},
		{"new at threshold", FSRSPhaseNew, 21.0, TierInProgress},
		{"new above threshold", FSRSPhaseNew, 21.1, TierInProgress},
		{"learning below threshold", FSRSPhaseLearning, 20.9, TierInProgress},
		{"learning at threshold", FSRSPhaseLearning, 21.0, TierInProgress},
		{"learning above threshold", FSRSPhaseLearning, 21.1, TierInProgress},
		{"relearning below threshold", FSRSPhaseRelearning, 20.9, TierInProgress},
		{"relearning at threshold", FSRSPhaseRelearning, 21.0, TierInProgress},
		{"relearning above threshold", FSRSPhaseRelearning, 21.1, TierInProgress},
		// Review phase splits on the inclusive 21d boundary.
		{"review just below threshold", FSRSPhaseReview, 20.9, TierLearned},
		{"review at threshold is mature", FSRSPhaseReview, 21.0, TierMature},
		{"review just above threshold", FSRSPhaseReview, 21.1, TierMature},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			state := FSRSState{Phase: tc.phase, Stability: tc.stability}
			require.Equal(t, tc.want, ClassifyMastery(state, threshold),
				"phase=%d stability=%v", tc.phase, tc.stability)
		})
	}
}

// TestMatureStabilityDays_DefaultBoundary confirms the shipped default const
// buckets a Review card of exactly 21d stability as Mature.
func TestMatureStabilityDays_DefaultBoundary(t *testing.T) {
	t.Parallel()
	require.Equal(t, 21.0, MatureStabilityDays)

	atThreshold := FSRSState{Phase: FSRSPhaseReview, Stability: MatureStabilityDays}
	require.Equal(t, TierMature, ClassifyMastery(atThreshold, MatureStabilityDays))

	belowThreshold := FSRSState{Phase: FSRSPhaseReview, Stability: MatureStabilityDays - 0.1}
	require.Equal(t, TierLearned, ClassifyMastery(belowThreshold, MatureStabilityDays))
}
