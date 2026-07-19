package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"backend/internal/domain"
)

func TestFSRSSchedulerApplyIsPure(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	initial := domain.NewFSRSStateForNewCard(now.Add(-24 * time.Hour))
	before := initial

	got := NewFSRSScheduler().Apply(initial, domain.RatingEasy, now)

	require.Equal(t, before, initial)
	require.NotEqual(t, initial, got)
	require.Equal(t, now, got.LastReview)
	require.Equal(t, domain.RatingEasy, got.LastRating)
}

func TestFSRSScheduler_Apply_BackwardClockSkewClamped(t *testing.T) {
	t.Parallel()

	scheduler := NewFSRSScheduler()

	reviewAt := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	// First review promotes a new card into the Review phase, stamping
	// LastReview == reviewAt.
	reviewed := scheduler.Apply(
		domain.NewFSRSStateForNewCard(reviewAt.Add(-24*time.Hour)),
		domain.RatingEasy,
		reviewAt,
	)
	require.Equal(t, domain.FSRSPhaseReview, reviewed.Phase)
	require.Equal(t, reviewAt, reviewed.LastReview)

	// A backward clock step (now before LastReview) must clamp to LastReview, so
	// the result is byte-for-byte identical to reviewing exactly at LastReview.
	skewed := scheduler.Apply(reviewed, domain.RatingGood, reviewAt.Add(-time.Second))
	atLastReview := scheduler.Apply(reviewed, domain.RatingGood, reviewAt)

	require.Equal(t, atLastReview, skewed)
	require.Equal(t, 0, skewed.ElapsedDays)
	require.GreaterOrEqual(t, skewed.ScheduledDays, 1)
}

func TestFSRSSchedulerApplyGoldenTransitions(t *testing.T) {
	t.Parallel()

	reviewAt := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	base := domain.FSRSState{
		Due:           reviewAt.Add(-24 * time.Hour),
		Stability:     2.5,
		Difficulty:    5.0,
		ElapsedDays:   1,
		ScheduledDays: 1,
		Reps:          1,
		Lapses:        0,
		LastReview:    reviewAt.Add(-24 * time.Hour),
	}

	cases := []struct {
		name          string
		state         domain.FSRSPhase
		rating        domain.Rating
		dueHours      float64
		stability     float64
		difficulty    float64
		elapsedDays   int
		scheduledDays int
		reps          int
		lapses        int
		outState      domain.FSRSPhase
	}{
		// Long-term scheduling mode (EnableShortTerm=false): every rating schedules a
		// whole-day interval and lands in the Review phase — the sub-day Learning /
		// Relearning steps are skipped, so the learning/review/relearning rows for a
		// given rating collapse onto identical values.
		{"new_again", domain.FSRSPhaseNew, domain.RatingAgain, 24, 0.402550000000, 7.194900000000, 0, 1, 2, 0, domain.FSRSPhaseReview},
		{"new_hard", domain.FSRSPhaseNew, domain.RatingHard, 48, 1.183850000000, 6.488305268471, 0, 2, 2, 0, domain.FSRSPhaseReview},
		{"new_good", domain.FSRSPhaseNew, domain.RatingGood, 72, 3.173000000000, 5.282434422319, 0, 3, 2, 0, domain.FSRSPhaseReview},
		{"new_easy", domain.FSRSPhaseNew, domain.RatingEasy, 384, 15.691050000000, 3.224501589371, 0, 16, 2, 0, domain.FSRSPhaseReview},
		{"learning_again", domain.FSRSPhaseLearning, domain.RatingAgain, 24, 0.805907963439, 6.607035107311, 1, 1, 2, 1, domain.FSRSPhaseReview},
		{"learning_hard", domain.FSRSPhaseLearning, domain.RatingHard, 72, 3.167603585081, 5.799433907311, 1, 3, 2, 0, domain.FSRSPhaseReview},
		{"learning_good", domain.FSRSPhaseLearning, domain.RatingGood, 120, 5.383816782206, 4.991832707311, 1, 5, 2, 0, domain.FSRSPhaseReview},
		{"learning_easy", domain.FSRSPhaseLearning, domain.RatingEasy, 264, 11.122035415438, 4.184231507311, 1, 11, 2, 0, domain.FSRSPhaseReview},
		{"review_again", domain.FSRSPhaseReview, domain.RatingAgain, 24, 0.805907963439, 6.607035107311, 1, 1, 2, 1, domain.FSRSPhaseReview},
		{"review_hard", domain.FSRSPhaseReview, domain.RatingHard, 72, 3.167603585081, 5.799433907311, 1, 3, 2, 0, domain.FSRSPhaseReview},
		{"review_good", domain.FSRSPhaseReview, domain.RatingGood, 120, 5.383816782206, 4.991832707311, 1, 5, 2, 0, domain.FSRSPhaseReview},
		{"review_easy", domain.FSRSPhaseReview, domain.RatingEasy, 264, 11.122035415438, 4.184231507311, 1, 11, 2, 0, domain.FSRSPhaseReview},
		{"relearning_again", domain.FSRSPhaseRelearning, domain.RatingAgain, 24, 0.805907963439, 6.607035107311, 1, 1, 2, 1, domain.FSRSPhaseReview},
		{"relearning_hard", domain.FSRSPhaseRelearning, domain.RatingHard, 72, 3.167603585081, 5.799433907311, 1, 3, 2, 0, domain.FSRSPhaseReview},
		{"relearning_good", domain.FSRSPhaseRelearning, domain.RatingGood, 120, 5.383816782206, 4.991832707311, 1, 5, 2, 0, domain.FSRSPhaseReview},
		{"relearning_easy", domain.FSRSPhaseRelearning, domain.RatingEasy, 264, 11.122035415438, 4.184231507311, 1, 11, 2, 0, domain.FSRSPhaseReview},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			scheduler := NewFSRSScheduler()
			input := base
			input.Phase = tc.state

			got := scheduler.Apply(input, tc.rating, reviewAt)

			require.Equal(t, reviewAt, got.LastReview)
			require.InDelta(t, tc.dueHours, got.Due.Sub(reviewAt).Hours(), 0.001)
			require.InDelta(t, tc.stability, got.Stability, 0.000000001)
			require.InDelta(t, tc.difficulty, got.Difficulty, 0.000000001)
			require.Equal(t, tc.elapsedDays, got.ElapsedDays)
			require.Equal(t, tc.scheduledDays, got.ScheduledDays)
			require.Equal(t, tc.reps, got.Reps)
			require.Equal(t, tc.lapses, got.Lapses)
			require.Equal(t, tc.outState, got.Phase)
			require.Equal(t, tc.rating, got.LastRating)
		})
	}
}
