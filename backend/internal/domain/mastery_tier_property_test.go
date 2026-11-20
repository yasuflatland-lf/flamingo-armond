package domain

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// TestClassifyMastery_Property_StabilityBands: with learned < mature, the tier is
// InProgress below learned, Mature at or above mature, Learned between; phase
// never matters; the tier is monotone in stability.
func TestClassifyMastery_Property_StabilityBands(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		learned := rapid.Float64Range(0.001, 100).Draw(t, "learned")
		mature := learned + rapid.Float64Range(0.001, 100).Draw(t, "gap")
		near := rapid.SampledFrom([]float64{learned, mature, math.Nextafter(learned, 0), math.Nextafter(mature, 0)})
		a := rapid.OneOf(rapid.Float64Range(0, 250), near).Draw(t, "a")
		b := rapid.OneOf(rapid.Float64Range(0, 250), near).Draw(t, "b")
		phase := FSRSPhase(rapid.IntRange(0, 3).Draw(t, "phase"))

		want := TierLearned
		switch {
		case a < learned:
			want = TierInProgress
		case a >= mature:
			want = TierMature
		}
		got := ClassifyMastery(FSRSState{Phase: phase, Stability: a}, learned, mature)
		require.Equal(t, want, got, "stability=%v learned=%v mature=%v", a, learned, mature)
		if a <= b {
			require.LessOrEqual(t, got, ClassifyMastery(FSRSState{Stability: b}, learned, mature))
		}
	})
}
