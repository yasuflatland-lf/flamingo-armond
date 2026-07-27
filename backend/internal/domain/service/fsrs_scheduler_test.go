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

// TestFSRSScheduler_Apply_InvalidPhasePanics pins the guard in front of the
// fsrs.State cast. The library dispatches on that value with no default arm, so
// an unrecognised phase would return a zero-valued scheduling result and
// silently wipe the card's state instead of failing. No caller can construct an
// invalid phase today — this is a programmer-error guard, so it panics rather
// than widening the domain.FSRSScheduler signature to return an error.
func TestFSRSScheduler_Apply_InvalidPhasePanics(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	scheduler := NewFSRSScheduler()

	for _, phase := range []domain.FSRSPhase{-1, 4, 99} {
		state := domain.NewFSRSStateForNewCard(now.Add(-24 * time.Hour))
		state.Phase = phase

		require.Panics(t, func() {
			scheduler.Apply(state, domain.RatingGood, now)
		}, "an unrecognised FSRSPhase must fail loudly, not schedule from a zero-valued result")
	}

	// Every recognised phase still schedules normally.
	for _, phase := range []domain.FSRSPhase{
		domain.FSRSPhaseNew,
		domain.FSRSPhaseLearning,
		domain.FSRSPhaseReview,
		domain.FSRSPhaseRelearning,
	} {
		state := domain.NewFSRSStateForNewCard(now.Add(-24 * time.Hour))
		state.Phase = phase

		require.NotPanics(t, func() {
			scheduler.Apply(state, domain.RatingGood, now)
		})
	}
}

// TestFSRSScheduler_Apply_InvalidRatingPanics pins the grade guard at the
// scheduler boundary. An out-of-range grade is a programmer error, so allowing
// one through would defer the failure to the scheduler library and force the
// domain.FSRSScheduler interface to represent an unreachable recoverable error.
func TestFSRSScheduler_Apply_InvalidRatingPanics(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	scheduler := NewFSRSScheduler()

	for _, rating := range []domain.Rating{0, 5} {
		state := domain.NewFSRSStateForNewCard(now.Add(-24 * time.Hour))

		require.Panics(t, func() {
			scheduler.Apply(state, rating, now)
		}, "an out-of-range Rating must fail loudly before reaching the scheduler library")
	}

	// Every recognised rating still schedules normally.
	for _, rating := range []domain.Rating{
		domain.RatingAgain,
		domain.RatingHard,
		domain.RatingGood,
		domain.RatingEasy,
	} {
		state := domain.NewFSRSStateForNewCard(now.Add(-24 * time.Hour))

		require.NotPanics(t, func() {
			scheduler.Apply(state, rating, now)
		})
	}
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

// TestFSRSScheduler_Apply_NeverSchedulesSubDayInterval pins the long-term-mode
// invariant: service.NewFSRSScheduler keeps EnableShortTerm false, so due is at
// least 24h after review. findDueCardsOn's rescue and filler windows and the
// day-granular replay guard in usecase/swipe.go depend on it; flipping the flag
// reopens the zero-credit repeat prevented by domain.rescueMinElapsed.
func TestFSRSScheduler_Apply_NeverSchedulesSubDayInterval(t *testing.T) {
	t.Parallel()

	reviewAt := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	ratings := []struct {
		name   string
		rating domain.Rating
	}{
		{"again", domain.RatingAgain},
		{"hard", domain.RatingHard},
		{"good", domain.RatingGood},
		{"easy", domain.RatingEasy},
	}
	phases := []struct {
		name  string
		phase domain.FSRSPhase
	}{
		{"new", domain.FSRSPhaseNew},
		{"learning", domain.FSRSPhaseLearning},
		{"review", domain.FSRSPhaseReview},
		{"relearning", domain.FSRSPhaseRelearning},
	}

	for _, phase := range phases {
		for _, rating := range ratings {
			t.Run(phase.name+"/"+rating.name, func(t *testing.T) {
				t.Parallel()

				state := domain.NewFSRSStateForNewCard(reviewAt.Add(-24 * time.Hour))
				state.Phase = phase.phase

				got := NewFSRSScheduler().Apply(state, rating.rating, reviewAt)

				require.GreaterOrEqual(t, got.Due.Sub(reviewAt), 24*time.Hour)
				require.GreaterOrEqual(t, got.ScheduledDays, 1)
			})
		}
	}
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
		{"new_again", domain.FSRSPhaseNew, domain.RatingAgain, 24, 0.212000000000, 6.413300000000, 0, 1, 2, 0, domain.FSRSPhaseReview},
		{"new_hard", domain.FSRSPhaseNew, domain.RatingHard, 48, 1.293100000000, 5.112170705601, 0, 2, 2, 0, domain.FSRSPhaseReview},
		{"new_good", domain.FSRSPhaseNew, domain.RatingGood, 72, 2.306500000000, 2.118103970459, 0, 3, 2, 0, domain.FSRSPhaseReview},
		{"new_easy", domain.FSRSPhaseNew, domain.RatingEasy, 192, 8.295600000000, 1.000000000000, 0, 8, 2, 0, domain.FSRSPhaseReview},
		{"learning_again", domain.FSRSPhaseLearning, domain.RatingAgain, 24, 0.569000576174, 8.341762369297, 1, 1, 2, 1, domain.FSRSPhaseReview},
		{"learning_hard", domain.FSRSPhaseLearning, domain.RatingHard, 120, 4.533552294443, 6.665995369297, 1, 5, 2, 0, domain.FSRSPhaseReview},
		{"learning_good", domain.FSRSPhaseLearning, domain.RatingGood, 144, 5.881363974797, 4.990228369297, 1, 6, 2, 0, domain.FSRSPhaseReview},
		{"learning_easy", domain.FSRSPhaseLearning, domain.RatingEasy, 216, 8.832956588396, 3.314461369297, 1, 9, 2, 0, domain.FSRSPhaseReview},
		{"review_again", domain.FSRSPhaseReview, domain.RatingAgain, 24, 0.569000576174, 8.341762369297, 1, 1, 2, 1, domain.FSRSPhaseReview},
		{"review_hard", domain.FSRSPhaseReview, domain.RatingHard, 120, 4.533552294443, 6.665995369297, 1, 5, 2, 0, domain.FSRSPhaseReview},
		{"review_good", domain.FSRSPhaseReview, domain.RatingGood, 144, 5.881363974797, 4.990228369297, 1, 6, 2, 0, domain.FSRSPhaseReview},
		{"review_easy", domain.FSRSPhaseReview, domain.RatingEasy, 216, 8.832956588396, 3.314461369297, 1, 9, 2, 0, domain.FSRSPhaseReview},
		{"relearning_again", domain.FSRSPhaseRelearning, domain.RatingAgain, 24, 0.569000576174, 8.341762369297, 1, 1, 2, 1, domain.FSRSPhaseReview},
		{"relearning_hard", domain.FSRSPhaseRelearning, domain.RatingHard, 120, 4.533552294443, 6.665995369297, 1, 5, 2, 0, domain.FSRSPhaseReview},
		{"relearning_good", domain.FSRSPhaseRelearning, domain.RatingGood, 144, 5.881363974797, 4.990228369297, 1, 6, 2, 0, domain.FSRSPhaseReview},
		{"relearning_easy", domain.FSRSPhaseRelearning, domain.RatingEasy, 216, 8.832956588396, 3.314461369297, 1, 9, 2, 0, domain.FSRSPhaseReview},
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
