package service

import (
	"time"

	"backend/internal/domain"
)

const (
	ModeDifficult = 0
	ModeDefault   = 1
	ModeGood      = 2
	ModeEasy      = 3
	ModeInWhile   = 4

	MinReviewsForModeCalculation = 20
)

type PerformanceMetrics struct {
	SuccessRate   float64
	AvgDifficulty float64
	RetentionRate float64
	StudyStreak   int
	LapseRate     float64
	ReviewCount   int
}

func ComputeMetrics(swipes []domain.SwipeRecord, now time.Time) PerformanceMetrics {
	if len(swipes) == 0 {
		return PerformanceMetrics{
			SuccessRate:   0.5,
			AvgDifficulty: 0.5,
			RetentionRate: 0.5,
		}
	}

	successes := 0
	onTime := 0
	lapses := 0
	reviews := 0
	difficultySum := 0.0
	daysSeen := make(map[string]struct{}, len(swipes))

	for _, swipe := range swipes {
		if swipe.Rating >= domain.RatingGood {
			successes++
		}
		if swipe.StateAfter.ElapsedDays <= swipe.StateAfter.ScheduledDays {
			onTime++
		}
		if isKnownCardReview(swipe) {
			reviews++
			if swipe.Rating == domain.RatingAgain {
				lapses++
			}
		}

		difficultySum += normalizedDifficulty(swipe.StateAfter.Difficulty)
		daysSeen[swipe.ReviewedAt.In(now.Location()).Format(time.DateOnly)] = struct{}{}
	}

	return PerformanceMetrics{
		SuccessRate:   float64(successes) / float64(len(swipes)),
		AvgDifficulty: difficultySum / float64(len(swipes)),
		RetentionRate: float64(onTime) / float64(len(swipes)),
		StudyStreak:   studyStreak(daysSeen, now),
		LapseRate:     ratio(lapses, reviews),
		ReviewCount:   len(swipes),
	}
}

func ModeFromMetrics(m PerformanceMetrics) int {
	if m.ReviewCount < MinReviewsForModeCalculation {
		return ModeDefault
	}

	mode := ModeInWhile
	switch {
	case m.SuccessRate < 0.60:
		mode = ModeDifficult
	case m.SuccessRate < 0.75:
		mode = ModeDefault
	case m.SuccessRate < 0.85:
		mode = ModeGood
	case m.SuccessRate < 0.95:
		mode = ModeEasy
	}

	if m.AvgDifficulty >= 0.7 {
		mode--
	} else if m.AvgDifficulty <= 0.3 {
		mode++
	}

	return clampMode(mode)
}

func normalizedDifficulty(difficulty float64) float64 {
	if difficulty > 1 {
		difficulty = difficulty / 10
	}
	if difficulty < 0 {
		return 0
	}
	if difficulty > 1 {
		return 1
	}
	return difficulty
}

func isKnownCardReview(swipe domain.SwipeRecord) bool {
	if swipe.StateAfter.State == domain.FSRSStateReview {
		return true
	}
	return swipe.Rating == domain.RatingAgain &&
		swipe.StateAfter.State == domain.FSRSStateRelearning &&
		swipe.StateAfter.Lapses > 0
}

func studyStreak(daysSeen map[string]struct{}, now time.Time) int {
	streak := 0
	for day := startOfDay(now); ; day = day.AddDate(0, 0, -1) {
		if _, ok := daysSeen[day.Format(time.DateOnly)]; !ok {
			return streak
		}
		streak++
	}
}

func startOfDay(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, t.Location())
}

func ratio(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

func clampMode(mode int) int {
	if mode < ModeDifficult {
		return ModeDifficult
	}
	if mode > ModeInWhile {
		return ModeInWhile
	}
	return mode
}
