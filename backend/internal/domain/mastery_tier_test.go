package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
