package service

import (
	"math"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"backend/internal/domain"
)

func TestNewFSRSScheduler_KeepsLongTermMode(t *testing.T) {
	t.Parallel()

	algo := NewFSRSScheduler().algo

	// These assertions defend against the measured silent fallback caused by
	// RequestRetention = 0, MaximumInterval = 0 or 73000, and a NaN weight.
	require.False(t, algo.EnableShortTerm)
	require.Equal(t, 0.9, algo.RequestRetention)
	require.Equal(t, 36500.0, algo.MaximumInterval)
	require.Nil(t, algo.LearningSteps)
	require.Nil(t, algo.RelearningSteps)
}

func TestFSRSScheduler_Apply_OutputAlwaysSatisfiesDomainGuards(t *testing.T) {
	t.Parallel()

	const iterations = 2000

	rng := rand.New(rand.NewPCG(1, 2))
	scheduler := NewFSRSScheduler()
	now := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	executed := 0

	for i := 0; i < iterations; i++ {
		stability := math.Exp(math.Log(0.001) + rng.Float64()*(math.Log(36500)-math.Log(0.001)))
		lastReview := now.Add(-time.Duration(rng.IntN(401)) * 24 * time.Hour)
		dueOffsetDays := rng.IntN(401)
		state := domain.FSRSState{
			Due:           lastReview.Add(time.Duration(dueOffsetDays) * 24 * time.Hour),
			Stability:     stability,
			Difficulty:    domain.MinDifficulty + rng.Float64()*(domain.MaxDifficulty-domain.MinDifficulty),
			ScheduledDays: dueOffsetDays,
			Reps:          rng.IntN(5001),
			Lapses:        rng.IntN(5001),
			Phase:         domain.FSRSPhase(rng.IntN(4)),
			LastReview:    lastReview,
		}
		rating := domain.Rating(rng.IntN(4) + 1)

		out := scheduler.Apply(state, rating, now)
		executed++

		failureMessage := "iteration %d: input=%+v rating=%d now=%s"
		require.True(t, domain.IsValidStability(out.Stability), failureMessage, i, state, rating, now)
		require.True(t, domain.IsValidDifficulty(out.Difficulty), failureMessage, i, state, rating, now)
		require.GreaterOrEqual(t, out.Due.Sub(now), 24*time.Hour, failureMessage, i, state, rating, now)
		require.Equal(t, domain.FSRSPhaseReview, out.Phase, failureMessage, i, state, rating, now)
		require.True(t, out.LastReview.Equal(now), failureMessage, i, state, rating, now)
		require.Equal(t, rating, out.LastRating, failureMessage, i, state, rating, now)
	}

	require.Equal(t, iterations, executed, "property sweep must execute every iteration")
}
