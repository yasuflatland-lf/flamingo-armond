package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFSRSCardState_IsLearningPhase(t *testing.T) {
	t.Parallel()

	cases := []struct {
		state FSRSCardState
		want  bool
	}{
		{FSRSStateNew, false},
		{FSRSStateLearning, true},
		{FSRSStateReview, false},
		{FSRSStateRelearning, true},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, tc.state.IsLearningPhase(), "state %d", tc.state)
	}
}
