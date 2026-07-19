package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestClassifyMastery_Boundary pins both inclusive stability boundaries and
// confirms that legacy phase values do not affect the tier.
func TestClassifyMastery_Boundary(t *testing.T) {
	t.Parallel()

	const (
		learnedThreshold = 7.0
		matureThreshold  = 21.0
	)

	cases := []struct {
		name      string
		phase     FSRSPhase
		stability float64
		want      MasteryTier
	}{
		{"just below learned threshold", FSRSPhaseReview, 6.999, TierInProgress},
		{"at learned threshold", FSRSPhaseReview, 7.0, TierLearned},
		{"legacy learning phase in learned band", FSRSPhaseLearning, 15.0, TierLearned},
		{"just below mature threshold", FSRSPhaseReview, 20.999, TierLearned},
		{"at mature threshold", FSRSPhaseReview, 21.0, TierMature},
		{"above mature threshold", FSRSPhaseReview, 21.1, TierMature},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			state := FSRSState{Phase: tc.phase, Stability: tc.stability}
			require.Equal(t, tc.want, ClassifyMastery(state, learnedThreshold, matureThreshold),
				"phase=%d stability=%v", tc.phase, tc.stability)
		})
	}
}

// TestMasteryStabilityDays_DefaultBoundaries confirms the shipped thresholds.
func TestMasteryStabilityDays_DefaultBoundaries(t *testing.T) {
	t.Parallel()
	require.Equal(t, 7.0, LearnedStabilityDays)
	require.Equal(t, 21.0, MatureStabilityDays)

	atLearnedThreshold := FSRSState{Phase: FSRSPhaseReview, Stability: LearnedStabilityDays}
	require.Equal(t, TierLearned, ClassifyMastery(atLearnedThreshold, LearnedStabilityDays, MatureStabilityDays))

	atMatureThreshold := FSRSState{Phase: FSRSPhaseReview, Stability: MatureStabilityDays}
	require.Equal(t, TierMature, ClassifyMastery(atMatureThreshold, LearnedStabilityDays, MatureStabilityDays))
}
