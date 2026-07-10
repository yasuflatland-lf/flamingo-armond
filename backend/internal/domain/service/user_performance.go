package service

import (
	"time"

	"backend/internal/domain"
)

// PerformanceMode is the difficulty level inferred from a user's recent
// performance. Values intentionally span ModeDifficult (0) .. ModeMastered (4).
type PerformanceMode int

const (
	ModeDifficult PerformanceMode = 0
	ModeDefault   PerformanceMode = 1
	ModeGood      PerformanceMode = 2
	ModeEasy      PerformanceMode = 3
	ModeMastered   PerformanceMode = 4

	MinReviewsForModeCalculation = 20

	// Success-rate band thresholds for ModeFromMetrics. A success rate at or
	// above each threshold selects the named mode (or higher).
	defaultModeThreshold = 0.60
	goodModeThreshold    = 0.75
	easyModeThreshold    = 0.85
	masteredModeThreshold = 0.95

	// Average-difficulty nudges shift the band-selected mode by one step:
	// hard recent cards drop the mode, easy recent cards raise it.
	highDifficultyThreshold = 0.7
	lowDifficultyThreshold  = 0.3
)

// IsValid reports whether the mode is in the recognised range.
func (m PerformanceMode) IsValid() bool {
	return m >= ModeDifficult && m <= ModeMastered
}

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
		if swipe.Rating.IsSuccess() {
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
		daysSeen[domain.LearnDayKey(swipe.ReviewedAt)] = struct{}{}
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

func ModeFromMetrics(m PerformanceMetrics) PerformanceMode {
	if m.ReviewCount < MinReviewsForModeCalculation {
		return ModeDefault
	}

	mode := ModeMastered
	switch {
	case m.SuccessRate < defaultModeThreshold:
		mode = ModeDifficult
	case m.SuccessRate < goodModeThreshold:
		mode = ModeDefault
	case m.SuccessRate < easyModeThreshold:
		mode = ModeGood
	case m.SuccessRate < masteredModeThreshold:
		mode = ModeEasy
	}

	if m.AvgDifficulty >= highDifficultyThreshold {
		mode--
	} else if m.AvgDifficulty <= lowDifficultyThreshold {
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
	if swipe.StateAfter.Phase == domain.FSRSPhaseReview {
		return true
	}
	return swipe.Rating == domain.RatingAgain &&
		swipe.StateAfter.Phase == domain.FSRSPhaseRelearning &&
		swipe.StateAfter.Lapses > 0
}

// studyStreak counts consecutive JST learn-days ending at the current learn-day,
// breaking on the first day with no swipe. If the current learn-day has no
// swipe, the streak is 0.
func studyStreak(daysSeen map[string]struct{}, now time.Time) int {
	streak := 0
	for day := domain.StartOfLearnDay(now); ; day = day.AddDate(0, 0, -1) {
		if _, ok := daysSeen[domain.LearnDayKey(day)]; !ok {
			return streak
		}
		streak++
	}
}

func ratio(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

func clampMode(mode PerformanceMode) PerformanceMode {
	if mode < ModeDifficult {
		return ModeDifficult
	}
	if mode > ModeMastered {
		return ModeMastered
	}
	return mode
}
