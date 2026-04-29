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
		state         domain.FSRSCardState
		rating        domain.Rating
		dueHours      float64
		stability     float64
		difficulty    float64
		elapsedDays   int
		scheduledDays int
		reps          int
		lapses        int
		outState      domain.FSRSCardState
	}{
		{"new_again", domain.FSRSStateNew, domain.RatingAgain, 1.0 / 60.0, 0.402550000000, 7.194900000000, 0, 0, 2, 0, domain.FSRSStateLearning},
		{"new_hard", domain.FSRSStateNew, domain.RatingHard, 5.0 / 60.0, 1.183850000000, 6.488305268471, 0, 0, 2, 0, domain.FSRSStateLearning},
		{"new_good", domain.FSRSStateNew, domain.RatingGood, 10.0 / 60.0, 3.173000000000, 5.282434422319, 0, 0, 2, 0, domain.FSRSStateLearning},
		{"new_easy", domain.FSRSStateNew, domain.RatingEasy, 384, 15.691050000000, 3.224501589371, 0, 16, 2, 0, domain.FSRSStateReview},
		{"learning_again", domain.FSRSStateLearning, domain.RatingAgain, 5.0 / 60.0, 1.252571310484, 6.607035107311, 1, 0, 2, 0, domain.FSRSStateLearning},
		{"learning_hard", domain.FSRSStateLearning, domain.RatingHard, 10.0 / 60.0, 2.099603435952, 5.799433907311, 1, 0, 2, 0, domain.FSRSStateLearning},
		{"learning_good", domain.FSRSStateLearning, domain.RatingGood, 96, 3.519428036844, 4.991832707311, 1, 4, 2, 0, domain.FSRSStateReview},
		{"learning_easy", domain.FSRSStateLearning, domain.RatingEasy, 144, 5.899387234001, 4.184231507311, 1, 6, 2, 0, domain.FSRSStateReview},
		{"review_again", domain.FSRSStateReview, domain.RatingAgain, 5.0 / 60.0, 0.805907963439, 6.607035107311, 1, 0, 2, 1, domain.FSRSStateRelearning},
		{"review_hard", domain.FSRSStateReview, domain.RatingHard, 72, 3.167603585081, 5.799433907311, 1, 3, 2, 0, domain.FSRSStateReview},
		{"review_good", domain.FSRSStateReview, domain.RatingGood, 120, 5.383816782206, 4.991832707311, 1, 5, 2, 0, domain.FSRSStateReview},
		{"review_easy", domain.FSRSStateReview, domain.RatingEasy, 264, 11.122035415438, 4.184231507311, 1, 11, 2, 0, domain.FSRSStateReview},
		{"relearning_again", domain.FSRSStateRelearning, domain.RatingAgain, 5.0 / 60.0, 1.252571310484, 6.607035107311, 1, 0, 2, 0, domain.FSRSStateRelearning},
		{"relearning_hard", domain.FSRSStateRelearning, domain.RatingHard, 10.0 / 60.0, 2.099603435952, 5.799433907311, 1, 0, 2, 0, domain.FSRSStateRelearning},
		{"relearning_good", domain.FSRSStateRelearning, domain.RatingGood, 96, 3.519428036844, 4.991832707311, 1, 4, 2, 0, domain.FSRSStateReview},
		{"relearning_easy", domain.FSRSStateRelearning, domain.RatingEasy, 144, 5.899387234001, 4.184231507311, 1, 6, 2, 0, domain.FSRSStateReview},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			scheduler := NewFSRSScheduler()
			input := base
			input.State = tc.state

			got := scheduler.Apply(input, tc.rating, reviewAt)

			require.Equal(t, reviewAt, got.LastReview)
			require.InDelta(t, tc.dueHours, got.Due.Sub(reviewAt).Hours(), 0.001)
			require.InDelta(t, tc.stability, got.Stability, 0.000000001)
			require.InDelta(t, tc.difficulty, got.Difficulty, 0.000000001)
			require.Equal(t, tc.elapsedDays, got.ElapsedDays)
			require.Equal(t, tc.scheduledDays, got.ScheduledDays)
			require.Equal(t, tc.reps, got.Reps)
			require.Equal(t, tc.lapses, got.Lapses)
			require.Equal(t, tc.outState, got.State)
		})
	}
}
