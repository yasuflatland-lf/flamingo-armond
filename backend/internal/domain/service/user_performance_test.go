package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"backend/internal/domain"
)

func TestComputeMetrics(t *testing.T) {
	t.Parallel()

	// 03:00 UTC == 12:00 JST, safely inside one JST learn-day, so the
	// relative-day fixtures below stay on consecutive JST learn-days.
	now := time.Date(2026, 4, 30, 3, 0, 0, 0, time.UTC)

	cases := []struct {
		name   string
		swipes []domain.SwipeRecord
		want   PerformanceMetrics
	}{
		{
			name: "empty returns neutral defaults",
			want: PerformanceMetrics{
				SuccessRate:   0.5,
				AvgDifficulty: 0.5,
				RetentionRate: 0.5,
			},
		},
		{
			name: "success rate counts good and easy",
			swipes: []domain.SwipeRecord{
				swipe(domain.RatingAgain, now, state(5, 1, 1, domain.FSRSPhaseLearning)),
				swipe(domain.RatingHard, now, state(5, 1, 1, domain.FSRSPhaseLearning)),
				swipe(domain.RatingGood, now, state(5, 1, 1, domain.FSRSPhaseReview)),
				swipe(domain.RatingEasy, now, state(5, 1, 1, domain.FSRSPhaseReview)),
			},
			want: PerformanceMetrics{
				SuccessRate:   0.5,
				AvgDifficulty: 0.5,
				RetentionRate: 1,
				StudyStreak:   1,
				LapseRate:     0,
				ReviewCount:   4,
			},
		},
		{
			name: "average difficulty normalizes fsrs scale",
			swipes: []domain.SwipeRecord{
				swipe(domain.RatingGood, now, state(3, 1, 1, domain.FSRSPhaseReview)),
				swipe(domain.RatingGood, now, state(7, 1, 1, domain.FSRSPhaseReview)),
			},
			want: PerformanceMetrics{
				SuccessRate:   1,
				AvgDifficulty: 0.5,
				RetentionRate: 1,
				StudyStreak:   1,
				LapseRate:     0,
				ReviewCount:   2,
			},
		},
		{
			name: "retention rate counts reviews completed on time",
			swipes: []domain.SwipeRecord{
				swipe(domain.RatingGood, now, state(5, 2, 2, domain.FSRSPhaseReview)),
				swipe(domain.RatingGood, now, state(5, 3, 2, domain.FSRSPhaseReview)),
				swipe(domain.RatingGood, now, state(5, 1, 2, domain.FSRSPhaseReview)),
			},
			want: PerformanceMetrics{
				SuccessRate:   1,
				AvgDifficulty: 0.5,
				RetentionRate: 2.0 / 3.0,
				StudyStreak:   1,
				LapseRate:     0,
				ReviewCount:   3,
			},
		},
		{
			name: "study streak counts consecutive days from provided now",
			swipes: []domain.SwipeRecord{
				swipe(domain.RatingGood, now.Add(-2*time.Hour), state(5, 1, 1, domain.FSRSPhaseReview)),
				swipe(domain.RatingGood, now.AddDate(0, 0, -1), state(5, 1, 1, domain.FSRSPhaseReview)),
				swipe(domain.RatingGood, now.AddDate(0, 0, -2), state(5, 1, 1, domain.FSRSPhaseReview)),
				swipe(domain.RatingGood, now.AddDate(0, 0, -4), state(5, 1, 1, domain.FSRSPhaseReview)),
			},
			want: PerformanceMetrics{
				SuccessRate:   1,
				AvgDifficulty: 0.5,
				RetentionRate: 1,
				StudyStreak:   3,
				LapseRate:     0,
				ReviewCount:   4,
			},
		},
		{
			name: "lapse rate counts again ratings on known cards",
			swipes: []domain.SwipeRecord{
				swipe(domain.RatingAgain, now, stateWithLapses(5, 1, 1, domain.FSRSPhaseRelearning, 1)),
				swipe(domain.RatingGood, now, state(5, 1, 1, domain.FSRSPhaseReview)),
				swipe(domain.RatingAgain, now, state(5, 1, 1, domain.FSRSPhaseLearning)),
			},
			want: PerformanceMetrics{
				SuccessRate:   1.0 / 3.0,
				AvgDifficulty: 0.5,
				RetentionRate: 1,
				StudyStreak:   1,
				LapseRate:     0.5,
				ReviewCount:   3,
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := ComputeMetrics(tc.swipes, now)

			require.InDelta(t, tc.want.SuccessRate, got.SuccessRate, 0.000000001)
			require.InDelta(t, tc.want.AvgDifficulty, got.AvgDifficulty, 0.000000001)
			require.InDelta(t, tc.want.RetentionRate, got.RetentionRate, 0.000000001)
			require.Equal(t, tc.want.StudyStreak, got.StudyStreak)
			require.InDelta(t, tc.want.LapseRate, got.LapseRate, 0.000000001)
			require.Equal(t, tc.want.ReviewCount, got.ReviewCount)
		})
	}
}

func TestComputeWindowedMetrics(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 18, 3, 0, 0, 0, time.UTC)
	reviewState := state(5, 1, 1, domain.FSRSPhaseReview)

	t.Run("includes exact cutoff boundaries", func(t *testing.T) {
		t.Parallel()

		swipes := []domain.SwipeRecord{
			swipe(domain.RatingGood, now, reviewState),
			swipe(domain.RatingGood, now.AddDate(0, 0, -7), reviewState),
			swipe(domain.RatingGood, now.AddDate(0, 0, -7).Add(-time.Nanosecond), reviewState),
			swipe(domain.RatingGood, now.AddDate(0, 0, -30), reviewState),
			swipe(domain.RatingGood, now.AddDate(0, 0, -30).Add(-time.Nanosecond), reviewState),
		}

		got := ComputeWindowedMetrics(swipes, now)

		require.Equal(t, 5, got.Days365.ReviewCount)
		require.Equal(t, 4, got.Days30.ReviewCount)
		require.Equal(t, 2, got.Days7.ReviewCount)
	})

	t.Run("pins shorter-window streaks to the 365-day streak", func(t *testing.T) {
		t.Parallel()

		swipes := make([]domain.SwipeRecord, 0, 40)
		for daysAgo := 0; daysAgo < 40; daysAgo++ {
			swipes = append(swipes, swipe(domain.RatingGood, now.AddDate(0, 0, -daysAgo), reviewState))
		}

		got := ComputeWindowedMetrics(swipes, now)

		require.Equal(t, 40, got.Days365.StudyStreak)
		require.Equal(t, got.Days365.StudyStreak, got.Days30.StudyStreak)
		require.Equal(t, got.Days365.StudyStreak, got.Days7.StudyStreak)
	})

	t.Run("keeps empty shorter windows neutral with a non-empty 365-day set", func(t *testing.T) {
		t.Parallel()

		got := ComputeWindowedMetrics([]domain.SwipeRecord{
			swipe(domain.RatingEasy, now.AddDate(0, 0, -31), reviewState),
		}, now)

		require.Equal(t, 1, got.Days365.ReviewCount)
		require.Equal(t, PerformanceMetrics{SuccessRate: 0.5, AvgDifficulty: 0.5, RetentionRate: 0.5}, got.Days30)
		require.Equal(t, PerformanceMetrics{SuccessRate: 0.5, AvgDifficulty: 0.5, RetentionRate: 0.5}, got.Days7)
	})

	t.Run("returns neutral snapshots for fully empty input", func(t *testing.T) {
		t.Parallel()

		neutral := PerformanceMetrics{SuccessRate: 0.5, AvgDifficulty: 0.5, RetentionRate: 0.5}
		require.Equal(t, WindowedMetrics{Days365: neutral, Days30: neutral, Days7: neutral}, ComputeWindowedMetrics(nil, now))
	})
}

// TestComputeMetrics_JSTLearnDayBoundary proves the study streak and distinct-day
// counting bucket on the canonical JST learn-day, not on UTC. A swipe at 00:30 JST
// (= the previous day 15:30 UTC) must count on its JST calendar day, and a streak
// spanning that boundary must stay unbroken. Under UTC bucketing the same fixtures
// would yield a streak of 0.
func TestComputeMetrics_JSTLearnDayBoundary(t *testing.T) {
	t.Parallel()

	// now is 2026-05-02 10:00 JST (= 2026-05-02 01:00 UTC); the current learn-day is 2026-05-02.
	now := time.Date(2026, 5, 2, 1, 0, 0, 0, time.UTC)

	swipes := []domain.SwipeRecord{
		// 2026-05-02 00:30 JST: counts on 2026-05-02, not the previous UTC day.
		swipe(domain.RatingEasy, time.Date(2026, 5, 1, 15, 30, 0, 0, time.UTC), state(5, 1, 1, domain.FSRSPhaseReview)),
		// 2026-05-01 00:30 JST: the previous learn-day.
		swipe(domain.RatingEasy, time.Date(2026, 4, 30, 15, 30, 0, 0, time.UTC), state(5, 1, 1, domain.FSRSPhaseReview)),
	}

	got := ComputeMetrics(swipes, now)

	require.Equal(t, 2, got.StudyStreak)
}

func TestModeFromMetricsThresholdBoundaries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		reviewCount int
		successRate float64
		want        PerformanceMode
	}{
		{"under minimum reviews defaults", 19, 1, ModeDefault},
		{"below difficult threshold", 20, 0.599, ModeDifficult},
		{"at default threshold", 20, 0.60, ModeDefault},
		{"below good threshold", 20, 0.749, ModeDefault},
		{"at good threshold", 20, 0.75, ModeGood},
		{"below easy threshold", 20, 0.849, ModeGood},
		{"at easy threshold", 20, 0.85, ModeEasy},
		{"below mastered threshold", 20, 0.949, ModeEasy},
		{"at mastered threshold", 20, 0.95, ModeMastered},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := ModeFromMetrics(PerformanceMetrics{
				SuccessRate:   tc.successRate,
				AvgDifficulty: 0.5,
				ReviewCount:   tc.reviewCount,
			})

			require.Equal(t, tc.want, got)
		})
	}
}

func TestModeFromMetricsDifficultyAdjustments(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		successRate   float64
		avgDifficulty float64
		want          PerformanceMode
	}{
		{"high difficulty decreases mode", 0.80, 0.70, ModeDefault},
		{"low difficulty increases mode", 0.80, 0.30, ModeEasy},
		{"neutral difficulty leaves mode", 0.80, 0.50, ModeGood},
		{"high difficulty clamps at difficult", 0.50, 0.90, ModeDifficult},
		{"low difficulty clamps at mastered", 0.99, 0.10, ModeMastered},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := ModeFromMetrics(PerformanceMetrics{
				SuccessRate:   tc.successRate,
				AvgDifficulty: tc.avgDifficulty,
				ReviewCount:   MinReviewsForModeCalculation,
			})

			require.Equal(t, tc.want, got)
		})
	}
}

// TestNormalizedDifficulty pins the difficulty/10 mapping at its boundary
// values. The FSRS floor of exactly 1.0 must normalize to 0.1 (the
// low-difficulty band), not 1.0 — the bug that inverted the mode for mastered
// cards left a strict `> 1` guard that skipped the division at the floor.
func TestNormalizedDifficulty(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		difficulty float64
		want       float64
	}{
		{"fsrs floor maps to low band", 1.0, 0.1},
		{"just above floor", 1.5, 0.15},
		{"midscale", 5.0, 0.5},
		{"fsrs ceiling maps to one", 10.0, 1.0},
		{"zero maps to zero", 0.0, 0.0},
		{"negative clamps to zero", -3.0, 0.0},
		{"above ceiling clamps to one", 12.0, 1.0},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			require.InDelta(t, tc.want, normalizedDifficulty(tc.difficulty), 0.000000001)
		})
	}
}

// TestNormalizedDifficultyMonotonic proves the normalization is non-decreasing
// as the stored difficulty rises across the full 1..10 FSRS scale, so a harder
// card never normalizes lower than an easier one.
func TestNormalizedDifficultyMonotonic(t *testing.T) {
	t.Parallel()

	prev := normalizedDifficulty(1.0)
	for d := 1.0; d <= 10.0; d += 0.5 {
		got := normalizedDifficulty(d)
		require.GreaterOrEqualf(t, got, prev,
			"normalizedDifficulty must be non-decreasing: difficulty %.1f gave %.4f after %.4f", d, got, prev)
		prev = got
	}
}

// TestModeFromMetricsFSRSFloorNotDecremented drives a 20-swipe history whose
// cards all sit at the FSRS difficulty floor of 1.0. The mastered cards must
// normalize into the low-difficulty band (0.1) and raise the mode, never trip
// the high-difficulty threshold and decrement it below the success-rate band.
func TestModeFromMetricsFSRSFloorNotDecremented(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 30, 3, 0, 0, 0, time.UTC)

	swipes := make([]domain.SwipeRecord, 0, MinReviewsForModeCalculation)
	for i := 0; i < 16; i++ {
		swipes = append(swipes, swipe(domain.RatingGood, now, state(1.0, 1, 1, domain.FSRSPhaseReview)))
	}
	for i := 0; i < 4; i++ {
		swipes = append(swipes, swipe(domain.RatingAgain, now, state(1.0, 1, 1, domain.FSRSPhaseReview)))
	}

	metrics := ComputeMetrics(swipes, now)
	// 16/20 successes selects the Good success-rate band (0.75 <= rate < 0.85).
	require.InDelta(t, 0.80, metrics.SuccessRate, 0.000000001)
	// All cards at the floor normalize to 0.1, the low-difficulty band.
	require.InDelta(t, 0.10, metrics.AvgDifficulty, 0.000000001)

	got := ModeFromMetrics(metrics)
	// The floor difficulty is <= 0.3, so the mode is raised to ModeEasy; it must
	// not be decremented below the Good band the success rate selected.
	require.GreaterOrEqual(t, int(got), int(ModeGood))
	require.Equal(t, ModeEasy, got)
}

func TestPerformanceModeIsValid(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		mode PerformanceMode
		want bool
	}{
		{"below range", -1, false},
		{"difficult lower bound", ModeDifficult, true},
		{"mastered upper bound", ModeMastered, true},
		{"above range", 5, false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tc.want, tc.mode.IsValid())
		})
	}
}

func swipe(rating domain.Rating, reviewedAt time.Time, stateAfter domain.FSRSState) domain.SwipeRecord {
	return domain.SwipeRecord{
		Rating:     rating,
		ReviewedAt: reviewedAt,
		StateAfter: stateAfter,
	}
}

func state(difficulty float64, elapsedDays int, scheduledDays int, cardState domain.FSRSPhase) domain.FSRSState {
	return stateWithLapses(difficulty, elapsedDays, scheduledDays, cardState, 0)
}

func stateWithLapses(
	difficulty float64,
	elapsedDays int,
	scheduledDays int,
	cardState domain.FSRSPhase,
	lapses int,
) domain.FSRSState {
	return domain.FSRSState{
		Difficulty:    difficulty,
		ElapsedDays:   elapsedDays,
		ScheduledDays: scheduledDays,
		Lapses:        lapses,
		Phase:         cardState,
	}
}
