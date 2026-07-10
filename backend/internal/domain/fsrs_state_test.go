package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFSRSCardState_IsLearningPhase(t *testing.T) {
	t.Parallel()

	cases := []struct {
		state FSRSPhase
		want  bool
	}{
		{FSRSPhaseNew, false},
		{FSRSPhaseLearning, true},
		{FSRSPhaseReview, false},
		{FSRSPhaseRelearning, true},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, tc.state.IsLearningPhase(), "state %d", tc.state)
	}
}
