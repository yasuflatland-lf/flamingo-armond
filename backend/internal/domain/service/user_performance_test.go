package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"backend/internal/domain"
)

func TestComputeMetrics(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 30, 15, 0, 0, 0, time.UTC)

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
				swipe(domain.RatingAgain, now, state(5, 1, 1, domain.FSRSStateLearning)),
				swipe(domain.RatingHard, now, state(5, 1, 1, domain.FSRSStateLearning)),
				swipe(domain.RatingGood, now, state(5, 1, 1, domain.FSRSStateReview)),
				swipe(domain.RatingEasy, now, state(5, 1, 1, domain.FSRSStateReview)),
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
				swipe(domain.RatingGood, now, state(3, 1, 1, domain.FSRSStateReview)),
				swipe(domain.RatingGood, now, state(7, 1, 1, domain.FSRSStateReview)),
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
				swipe(domain.RatingGood, now, state(5, 2, 2, domain.FSRSStateReview)),
				swipe(domain.RatingGood, now, state(5, 3, 2, domain.FSRSStateReview)),
				swipe(domain.RatingGood, now, state(5, 1, 2, domain.FSRSStateReview)),
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
				swipe(domain.RatingGood, now.Add(-2*time.Hour), state(5, 1, 1, domain.FSRSStateReview)),
				swipe(domain.RatingGood, now.AddDate(0, 0, -1), state(5, 1, 1, domain.FSRSStateReview)),
				swipe(domain.RatingGood, now.AddDate(0, 0, -2), state(5, 1, 1, domain.FSRSStateReview)),
				swipe(domain.RatingGood, now.AddDate(0, 0, -4), state(5, 1, 1, domain.FSRSStateReview)),
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
				swipe(domain.RatingAgain, now, stateWithLapses(5, 1, 1, domain.FSRSStateRelearning, 1)),
				swipe(domain.RatingGood, now, state(5, 1, 1, domain.FSRSStateReview)),
				swipe(domain.RatingAgain, now, state(5, 1, 1, domain.FSRSStateLearning)),
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

func TestModeFromMetricsThresholdBoundaries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		reviewCount int
		successRate float64
		want        int
	}{
		{"under minimum reviews defaults", 19, 1, ModeDefault},
		{"below difficult threshold", 20, 0.599, ModeDifficult},
		{"at default threshold", 20, 0.60, ModeDefault},
		{"below good threshold", 20, 0.749, ModeDefault},
		{"at good threshold", 20, 0.75, ModeGood},
		{"below easy threshold", 20, 0.849, ModeGood},
		{"at easy threshold", 20, 0.85, ModeEasy},
		{"below in while threshold", 20, 0.949, ModeEasy},
		{"at in while threshold", 20, 0.95, ModeInWhile},
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
		want          int
	}{
		{"high difficulty decreases mode", 0.80, 0.70, ModeDefault},
		{"low difficulty increases mode", 0.80, 0.30, ModeEasy},
		{"neutral difficulty leaves mode", 0.80, 0.50, ModeGood},
		{"high difficulty clamps at difficult", 0.50, 0.90, ModeDifficult},
		{"low difficulty clamps at in while", 0.99, 0.10, ModeInWhile},
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

func swipe(rating domain.Rating, reviewedAt time.Time, stateAfter domain.FSRSState) domain.SwipeRecord {
	return domain.SwipeRecord{
		Rating:     rating,
		ReviewedAt: reviewedAt,
		StateAfter: stateAfter,
	}
}

func state(difficulty float64, elapsedDays int, scheduledDays int, cardState domain.FSRSCardState) domain.FSRSState {
	return stateWithLapses(difficulty, elapsedDays, scheduledDays, cardState, 0)
}

func stateWithLapses(
	difficulty float64,
	elapsedDays int,
	scheduledDays int,
	cardState domain.FSRSCardState,
	lapses int,
) domain.FSRSState {
	return domain.FSRSState{
		Difficulty:    difficulty,
		ElapsedDays:   elapsedDays,
		ScheduledDays: scheduledDays,
		Lapses:        lapses,
		State:         cardState,
	}
}
