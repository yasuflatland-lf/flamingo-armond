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
				LapseRate:     0.5,
			},
		},
		{
			// A Learning->Easy graduation lands in StateAfter.Phase == Review, but
			// its pre-swipe phase was Learning, so it is not a review of an
			// already-learned card: reviews stays 0 and both scoped rates are 0.
			name: "new-format graduation swipe is excluded from reviews",
			swipes: []domain.SwipeRecord{
				swipeBefore(domain.RatingEasy, now, state(5, 3, domain.FSRSPhaseReview), domain.FSRSPhaseLearning, now),
			},
			want: PerformanceMetrics{
				SuccessRate:   1,
				AvgDifficulty: 0.5,
				RetentionRate: 0,
				StudyStreak:   1,
				LapseRate:     0,
				ReviewCount:   1,
			},
		},
		{
			// A repeated Again while already Relearning is neither a lapse (FSRS
			// increments Lapses only on Review->Again) nor a review, because the
			// pre-swipe phase was Relearning, not Review.
			name: "new-format relearning re-fail is excluded from reviews and lapses",
			swipes: []domain.SwipeRecord{
				swipeBefore(domain.RatingAgain, now, stateWithLapses(5, 0, domain.FSRSPhaseRelearning, 2), domain.FSRSPhaseRelearning, now),
			},
			want: PerformanceMetrics{
				SuccessRate:   0,
				AvgDifficulty: 0.5,
				RetentionRate: 0,
				StudyStreak:   1,
				LapseRate:     0,
				ReviewCount:   1,
			},
		},
		{
			// Review->Again is the one FSRS lapse transition: it counts as a lapse
			// once the pre-swipe stability is learned and, being an Again, is never
			// an on-time recall.
			name: "new-format review again is a lapse and not a retention",
			swipes: []domain.SwipeRecord{
				swipeBefore(domain.RatingAgain, now, stateWithLapses(5, 0, domain.FSRSPhaseRelearning, 1), domain.FSRSPhaseReview, now),
			},
			want: PerformanceMetrics{
				SuccessRate:   0,
				AvgDifficulty: 0.5,
				RetentionRate: 0,
				StudyStreak:   1,
				LapseRate:     1,
				ReviewCount:   1,
			},
		},
		{
			// The card's scheduled interval reached 7 while its stability was still
			// 6.6, so the mastery tiles called it In progress. The stability
			// snapshot keeps the Again out of the lapse numerator and out of the
			// review denominator entirely.
			name: "sub-learned stability keeps an interval-seven lapse out of the rates",
			swipes: []domain.SwipeRecord{
				swipeStability(domain.RatingAgain, now, stateWithLapses(5, 0, domain.FSRSPhaseRelearning, 1), domain.FSRSPhaseReview, 6.6, now),
			},
			want: PerformanceMetrics{
				SuccessRate:   0,
				AvgDifficulty: 0.5,
				RetentionRate: 0,
				StudyStreak:   1,
				LapseRate:     0,
				ReviewCount:   1,
			},
		},
		{
			// Exactly at the learned boundary the gate is inclusive, matching
			// ClassifyMastery, so the review counts.
			name: "stability at the learned boundary counts the review",
			swipes: []domain.SwipeRecord{
				swipeStability(domain.RatingGood, now, state(5, 9, domain.FSRSPhaseReview), domain.FSRSPhaseReview, 7.0, now),
			},
			want: PerformanceMetrics{
				SuccessRate:   1,
				AvgDifficulty: 0.5,
				RetentionRate: 1,
				StudyStreak:   1,
				LapseRate:     0,
				ReviewCount:   1,
			},
		},
		{
			// Hard is not a SuccessRate success (IsSuccess is >= Good) but it IS a
			// recall: a Review+Hard within the pre-swipe interval is a retention,
			// so SuccessRate (0) and RetentionRate (1) diverge deliberately.
			name: "new-format review hard on time is a retention but not a success",
			swipes: []domain.SwipeRecord{
				swipeBefore(domain.RatingHard, now, state(5, 8, domain.FSRSPhaseReview), domain.FSRSPhaseReview, now),
			},
			want: PerformanceMetrics{
				SuccessRate:   0,
				AvgDifficulty: 0.5,
				RetentionRate: 1,
				StudyStreak:   1,
				LapseRate:     0,
				ReviewCount:   1,
			},
		},
		{
			// A successful review answered later than the interval it was due
			// within is a review, but not an on-time recall.
			name: "new-format review easy later than scheduled is a review but not a retention",
			swipes: []domain.SwipeRecord{
				swipeBefore(domain.RatingEasy, now, state(5, 10, domain.FSRSPhaseReview), domain.FSRSPhaseReview, now.Add(-time.Hour)),
			},
			want: PerformanceMetrics{
				SuccessRate:   1,
				AvgDifficulty: 0.5,
				RetentionRate: 0,
				StudyStreak:   1,
				LapseRate:     0,
				ReviewCount:   1,
			},
		},
		{
			// The old day comparison counted this successful review as on time
			// because its truncated elapsed-day count equalled the scheduled
			// interval. The due instant shows it was already late, so it is now
			// correctly excluded from retention.
			name: "new-format review just after due is not on time",
			swipes: []domain.SwipeRecord{
				swipeBefore(domain.RatingGood, now, state(5, 9, domain.FSRSPhaseReview), domain.FSRSPhaseReview, now.Add(-time.Nanosecond)),
			},
			want: PerformanceMetrics{
				SuccessRate:   1,
				AvgDifficulty: 0.5,
				RetentionRate: 0,
				StudyStreak:   1,
				LapseRate:     0,
				ReviewCount:   1,
			},
		},
		{
			// ReviewedAt == DueBefore exactly is on time (inclusive boundary).
			name: "new-format review at the exact due instant is on time",
			swipes: []domain.SwipeRecord{
				swipeBefore(domain.RatingGood, now, state(5, 9, domain.FSRSPhaseReview), domain.FSRSPhaseReview, now),
			},
			want: PerformanceMetrics{
				SuccessRate:   1,
				AvgDifficulty: 0.5,
				RetentionRate: 1,
				StudyStreak:   1,
				LapseRate:     0,
				ReviewCount:   1,
			},
		},
		{
			// New-card swipes (pre-swipe phase New) — including a New->Easy
			// graduation whose StateAfter.Phase is Review — are never reviews, so
			// they move only SuccessRate and ReviewCount.
			name: "new-format new-card swipes affect only success rate and review count",
			swipes: []domain.SwipeRecord{
				swipeBefore(domain.RatingAgain, now, state(5, 0, domain.FSRSPhaseLearning), domain.FSRSPhaseNew, now),
				swipeBefore(domain.RatingEasy, now, state(5, 1, domain.FSRSPhaseReview), domain.FSRSPhaseNew, now),
			},
			want: PerformanceMetrics{
				SuccessRate:   0.5,
				AvgDifficulty: 0.5,
				RetentionRate: 0,
				StudyStreak:   1,
				LapseRate:     0,
				ReviewCount:   2,
			},
		},
		{
			// Several new-format reviews plus one graduation: reviews is 3 (the
			// graduation is excluded from the denominator); one Again is the lapse
			// numerator; one on-time Hard is the retention numerator.
			name: "new-format mixed reviews scope both rates to the review denominator",
			swipes: []domain.SwipeRecord{
				swipeBefore(domain.RatingAgain, now, stateWithLapses(5, 0, domain.FSRSPhaseRelearning, 1), domain.FSRSPhaseReview, now),
				swipeBefore(domain.RatingHard, now, state(5, 8, domain.FSRSPhaseReview), domain.FSRSPhaseReview, now),
				swipeBefore(domain.RatingEasy, now, state(5, 10, domain.FSRSPhaseReview), domain.FSRSPhaseReview, now.Add(-time.Hour)),
				swipeBefore(domain.RatingEasy, now, state(5, 1, domain.FSRSPhaseReview), domain.FSRSPhaseLearning, now),
			},
			want: PerformanceMetrics{
				SuccessRate:   0.5,
				AvgDifficulty: 0.5,
				RetentionRate: 1.0 / 3.0,
				StudyStreak:   1,
				LapseRate:     1.0 / 3.0,
				ReviewCount:   4,
			},
		},
		{
			// Study streak counts consecutive JST learn-days back from now; the
			// gap at day -3 caps it at 3. All four rows are on-time new-format
			// reviews, so the scoped rates stay at 1 / 0.
			name: "study streak counts consecutive days from provided now",
			swipes: []domain.SwipeRecord{
				swipe(domain.RatingGood, now.Add(-2*time.Hour), state(5, 7, domain.FSRSPhaseReview)),
				swipe(domain.RatingGood, now.AddDate(0, 0, -1), state(5, 7, domain.FSRSPhaseReview)),
				swipe(domain.RatingGood, now.AddDate(0, 0, -2), state(5, 7, domain.FSRSPhaseReview)),
				swipe(domain.RatingGood, now.AddDate(0, 0, -4), state(5, 7, domain.FSRSPhaseReview)),
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

// TestComputeMetrics_KnownReviewCount pins the denominator behind RetentionRate
// and LapseRate as a separate wire field, and the two branches that leave it at
// zero: an empty window, where both rates carry the neutral 0.5 placeholder, and
// a window whose swipes all fail the known-card gate, where both rates are a
// real 0. Consumers distinguish the two by KnownReviewCount, not by ReviewCount.
func TestComputeMetrics_KnownReviewCount(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 30, 3, 0, 0, 0, time.UTC)

	t.Run("an empty window carries the placeholder on every rate", func(t *testing.T) {
		t.Parallel()

		got := ComputeMetrics(nil, now)

		require.InDelta(t, 0.5, got.SuccessRate, 0.000000001)
		require.InDelta(t, 0.5, got.AvgDifficulty, 0.000000001)
		require.InDelta(t, 0.5, got.RetentionRate, 0.000000001)
		require.InDelta(t, 0.5, got.LapseRate, 0.000000001)
		require.Equal(t, 0, got.ReviewCount)
		require.Equal(t, 0, got.KnownReviewCount)
	})

	t.Run("swipes without gated reviews leave the known-review count at zero", func(t *testing.T) {
		t.Parallel()

		got := ComputeMetrics([]domain.SwipeRecord{
			// New-card swipe and a Learning->Easy graduation: neither is a review
			// of an already-learned card.
			swipeBefore(domain.RatingAgain, now, state(5, 0, domain.FSRSPhaseLearning), domain.FSRSPhaseNew, now),
			swipeBefore(domain.RatingEasy, now, state(5, 1, domain.FSRSPhaseReview), domain.FSRSPhaseLearning, now),
		}, now)

		require.Equal(t, 2, got.ReviewCount)
		require.Equal(t, 0, got.KnownReviewCount)
		require.Zero(t, got.RetentionRate)
		require.Zero(t, got.LapseRate)
	})

	t.Run("known-review count equals the gated denominator", func(t *testing.T) {
		t.Parallel()

		got := ComputeMetrics([]domain.SwipeRecord{
			swipeBefore(domain.RatingAgain, now, stateWithLapses(5, 0, domain.FSRSPhaseRelearning, 1), domain.FSRSPhaseReview, now),
			swipeBefore(domain.RatingHard, now, state(5, 8, domain.FSRSPhaseReview), domain.FSRSPhaseReview, now),
			swipeBefore(domain.RatingEasy, now, state(5, 10, domain.FSRSPhaseReview), domain.FSRSPhaseReview, now.Add(-time.Hour)),
			swipeBefore(domain.RatingEasy, now, state(5, 1, domain.FSRSPhaseReview), domain.FSRSPhaseLearning, now),
		}, now)

		require.Equal(t, 4, got.ReviewCount)
		require.Equal(t, 3, got.KnownReviewCount)
		require.InDelta(t, 1.0/3.0, got.RetentionRate, 0.000000001)
		require.InDelta(t, 1.0/3.0, got.LapseRate, 0.000000001)
	})
}

func TestIsKnownCardReview(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		swipe domain.SwipeRecord
		want  bool
	}{
		{
			name: "review below the learned boundary is excluded",
			swipe: domain.SwipeRecord{
				PhaseBefore:     domain.FSRSPhaseReview,
				StabilityBefore: domain.LearnedStabilityDays - 0.1,
			},
			want: false,
		},
		{
			name: "review at the learned boundary is included",
			swipe: domain.SwipeRecord{
				PhaseBefore:     domain.FSRSPhaseReview,
				StabilityBefore: domain.LearnedStabilityDays,
			},
			want: true,
		},
		{
			name: "review above the learned boundary is included",
			swipe: domain.SwipeRecord{
				PhaseBefore:     domain.FSRSPhaseReview,
				StabilityBefore: domain.LearnedStabilityDays + 0.1,
			},
			want: true,
		},
		{
			name: "new phase is excluded with learned-band stability",
			swipe: domain.SwipeRecord{
				PhaseBefore:     domain.FSRSPhaseNew,
				StabilityBefore: domain.LearnedStabilityDays,
			},
			want: false,
		},
		{
			name: "learning phase is excluded with learned-band stability",
			swipe: domain.SwipeRecord{
				PhaseBefore:     domain.FSRSPhaseLearning,
				StabilityBefore: domain.LearnedStabilityDays,
			},
			want: false,
		},
		{
			name: "relearning phase is excluded with learned-band stability",
			swipe: domain.SwipeRecord{
				PhaseBefore:     domain.FSRSPhaseRelearning,
				StabilityBefore: domain.LearnedStabilityDays,
			},
			want: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, isKnownCardReview(tc.swipe))
		})
	}
}

func TestComputeWindowedMetrics(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 18, 3, 0, 0, 0, time.UTC)
	reviewState := state(5, 1, domain.FSRSPhaseReview)

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
		require.Equal(t, PerformanceMetrics{SuccessRate: 0.5, AvgDifficulty: 0.5, RetentionRate: 0.5, LapseRate: 0.5}, got.Days30)
		require.Equal(t, PerformanceMetrics{SuccessRate: 0.5, AvgDifficulty: 0.5, RetentionRate: 0.5, LapseRate: 0.5}, got.Days7)
	})

	t.Run("returns neutral snapshots for fully empty input", func(t *testing.T) {
		t.Parallel()

		neutral := PerformanceMetrics{SuccessRate: 0.5, AvgDifficulty: 0.5, RetentionRate: 0.5, LapseRate: 0.5}
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
		swipe(domain.RatingEasy, time.Date(2026, 5, 1, 15, 30, 0, 0, time.UTC), state(5, 1, domain.FSRSPhaseReview)),
		// 2026-05-01 00:30 JST: the previous learn-day.
		swipe(domain.RatingEasy, time.Date(2026, 4, 30, 15, 30, 0, 0, time.UTC), state(5, 1, domain.FSRSPhaseReview)),
	}

	got := ComputeMetrics(swipes, now)

	require.Equal(t, 2, got.StudyStreak)
}

// TestStudyStreak_ReturnsUnclampedRunLength pins the raw streak behaviour the
// stats usecase's cap exists for: given 366 consecutive JST learn-day keys
// ending at the current learn-day, the walk reports the full run length of 366.
// studyStreak applies no cap of its own — it stops only at the first missing
// day — so a run one longer than the reported 365-day maximum reaches the
// caller intact. That the inclusive [now-365d, now] fetch window can actually
// supply 366 learn-days is proven end to end by
// TestStatsUsecase_MyLearningStats_StreakCappedAtStatsWindowDays in
// backend/internal/usecase/stats_test.go, which seeds swipes across the
// inclusive cutoff and asserts every reported window clamps back to 365.
func TestStudyStreak_ReturnsUnclampedRunLength(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 18, 3, 0, 0, 0, time.UTC)
	daysSeen := make(map[string]struct{}, 366)
	for day := domain.StartOfLearnDay(now); len(daysSeen) < 366; day = day.AddDate(0, 0, -1) {
		daysSeen[domain.LearnDayKey(day)] = struct{}{}
	}

	require.Equal(t, 366, studyStreak(daysSeen, now))
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
		swipes = append(swipes, swipe(domain.RatingGood, now, state(1.0, 1, domain.FSRSPhaseReview)))
	}
	for i := 0; i < 4; i++ {
		swipes = append(swipes, swipe(domain.RatingAgain, now, state(1.0, 1, domain.FSRSPhaseReview)))
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

// swipe builds an on-time review with a learned-band pre-swipe snapshot.
func swipe(rating domain.Rating, reviewedAt time.Time, stateAfter domain.FSRSState) domain.SwipeRecord {
	return domain.SwipeRecord{
		Rating:          rating,
		ReviewedAt:      reviewedAt,
		StateAfter:      stateAfter,
		PhaseBefore:     domain.FSRSPhaseReview,
		StabilityBefore: domain.LearnedStabilityDays,
		DueBefore:       reviewedAt,
	}
}

// swipeStability builds a swipe record carrying the full pre-swipe snapshot,
// including the stability and due instant the card held before the rating was
// applied.
func swipeStability(
	rating domain.Rating,
	reviewedAt time.Time,
	stateAfter domain.FSRSState,
	phaseBefore domain.FSRSPhase,
	stabilityBefore float64,
	dueBefore time.Time,
) domain.SwipeRecord {
	sr := swipeBefore(rating, reviewedAt, stateAfter, phaseBefore, dueBefore)
	sr.StabilityBefore = stabilityBefore
	return sr
}

// swipeBefore builds a swipe record carrying the pre-swipe phase, learned-band
// stability, and due instant that the metrics layer reads independently of
// StateAfter.
func swipeBefore(
	rating domain.Rating,
	reviewedAt time.Time,
	stateAfter domain.FSRSState,
	phaseBefore domain.FSRSPhase,
	dueBefore time.Time,
) domain.SwipeRecord {
	return domain.SwipeRecord{
		Rating:          rating,
		ReviewedAt:      reviewedAt,
		StateAfter:      stateAfter,
		PhaseBefore:     phaseBefore,
		StabilityBefore: domain.LearnedStabilityDays,
		DueBefore:       dueBefore,
	}
}

func state(difficulty float64, scheduledDays int, cardState domain.FSRSPhase) domain.FSRSState {
	return stateWithLapses(difficulty, scheduledDays, cardState, 0)
}

func stateWithLapses(
	difficulty float64,
	scheduledDays int,
	cardState domain.FSRSPhase,
	lapses int,
) domain.FSRSState {
	return domain.FSRSState{
		Difficulty:    difficulty,
		ScheduledDays: scheduledDays,
		Lapses:        lapses,
		Phase:         cardState,
	}
}
