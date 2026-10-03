package service

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"backend/internal/domain"
)

// genFSRSInput draws a validated FSRSState and a review instant. Stability is
// log-uniform over the domain range with its bounds and the new-card
// placeholder mixed in; now is usually after LastReview (within 3 hours, days
// or years later) and occasionally up to an hour before it (backward clock step).
func genFSRSInput() *rapid.Generator[fsrsInput] {
	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	return rapid.Custom(func(t *rapid.T) fsrsInput {
		stability := rapid.OneOf(
			rapid.Map(rapid.Float64Range(math.Log(domain.MinStability), math.Log(domain.MaxStability)), math.Exp),
			rapid.SampledFrom([]float64{domain.MinStability, domain.MaxStability, domain.NewCardStability}),
		).Draw(t, "stability")
		last := base.Add(-time.Duration(rapid.Int64Range(0, int64(800*24*time.Hour)).Draw(t, "age")))
		var now time.Time
		switch rapid.IntRange(0, 3).Draw(t, "clock") {
		case 0:
			now = last.Add(time.Duration(rapid.Int64Range(0, int64(3*time.Hour)).Draw(t, "withinHours")))
		case 1:
			now = last.Add(-time.Duration(rapid.Int64Range(1, int64(time.Hour)).Draw(t, "backward")))
		default:
			now = last.Add(time.Duration(rapid.Int64Range(0, int64(800*24*time.Hour)).Draw(t, "elapsed")))
		}
		return fsrsInput{
			state: domain.FSRSState{
				Due:           last.Add(time.Duration(rapid.Int64Range(0, int64(400*24*time.Hour)).Draw(t, "due"))),
				Stability:     stability,
				Difficulty:    rapid.Float64Range(domain.MinDifficulty, domain.MaxDifficulty).Draw(t, "difficulty"),
				ScheduledDays: rapid.IntRange(0, 400).Draw(t, "scheduledDays"),
				Reps:          rapid.IntRange(0, 5000).Draw(t, "reps"),
				Lapses:        rapid.IntRange(0, 5000).Draw(t, "lapses"),
				Phase:         domain.FSRSPhase(rapid.IntRange(0, 3).Draw(t, "phase")),
				LastReview:    last,
			},
			now: now,
		}
	})
}

type fsrsInput struct {
	state domain.FSRSState
	now   time.Time
}

// TestFSRSScheduler_Apply_Property_Laws: for every validated input and rating the
// output is in range, Review phase, Reps+1, LastReview = max(now, input
// LastReview), Due >= LastReview+24h; and for one input, Hard <= Good <= Easy on
// stability and due, and Again <= Hard on stability unless the card is New.
func TestFSRSScheduler_Apply_Property_Laws(t *testing.T) {
	t.Parallel()
	s := NewFSRSScheduler()
	rapid.Check(t, func(t *rapid.T) {
		in := genFSRSInput().Draw(t, "in")
		effNow := in.now
		if effNow.Before(in.state.LastReview) {
			effNow = in.state.LastReview
		}
		var out [domain.RatingEasy + 1]domain.FSRSState
		for r := domain.RatingAgain; r <= domain.RatingEasy; r++ {
			o := s.Apply(in.state, r, in.now)
			out[r] = o
			require.True(t, domain.IsValidStability(o.Stability), "rating %d: %+v", r, o)
			require.True(t, domain.IsValidDifficulty(o.Difficulty), "rating %d: %+v", r, o)
			require.Equal(t, domain.FSRSPhaseReview, o.Phase)
			require.Equal(t, in.state.Reps+1, o.Reps)
			require.True(t, o.LastReview.Equal(effNow))
			require.GreaterOrEqual(t, o.Due.Sub(effNow), 24*time.Hour)
		}
		for r := domain.RatingHard; r < domain.RatingEasy; r++ {
			require.LessOrEqual(t, out[r].Stability, out[r+1].Stability, "stability rating %d vs %d", r, r+1)
			require.False(t, out[r].Due.After(out[r+1].Due), "due rating %d vs %d", r, r+1)
		}
		if in.state.Phase != domain.FSRSPhaseNew {
			require.LessOrEqual(t, out[domain.RatingAgain].Stability, out[domain.RatingHard].Stability)
		}
	})
}
