package service

import (
	"sort"
	"time"

	"github.com/rotisserie/eris"

	"backend/internal/domain"
)

// MasteryBreakdown is the disjoint three-tier count across all studied cards.
// InProgress + Learned + Mature == TotalStudied by construction.
type MasteryBreakdown struct{ InProgress, Learned, Mature, TotalStudied int }

// DeckMastery is a per-deck acquisition summary. LearnedCards and MatureCards
// are disjoint; acquired == LearnedCards + MatureCards, and acquired never
// exceeds TotalCards.
type DeckMastery struct {
	CardgroupID  string
	TotalCards   int
	LearnedCards int // disjoint: Review & stability < MatureStabilityDays
	MatureCards  int // disjoint: Review & stability >= MatureStabilityDays
}

// StrugglingCard identifies a card the learner struggles with (high lapses /
// low stability). CardID is hydrated to a Card by the resolver.
type StrugglingCard struct {
	CardID    string
	Lapses    int
	Stability float64
}

// AggregateMastery buckets studied FSRS stats into the global three-tier
// MasteryBreakdown and a per-deck DeckMastery split. This is the definition of
// "mastered" — a card's tier is decided by domain.ClassifyMastery, so the three
// tiers are disjoint and cover every studied card (InProgress + Learned + Mature
// == len(stats) == TotalStudied).
//
// deckCardTotals is the total card count per owned deck (only decks with at
// least one card). One DeckMastery is emitted for every entry, sorted by
// CardgroupID for a deterministic order, so a deck with zero studied cards still
// appears with LearnedCards == MatureCards == 0. Studied cards whose deck is
// absent from deckCardTotals still contribute to the global breakdown but have
// no DeckMastery denominator.
//
// The default arm is defensive: domain.ClassifyMastery only ever returns the
// three known tiers, so an unhandled tier signals a domain invariant break.
func AggregateMastery(stats []domain.FSRSStat, deckCardTotals map[string]int) (MasteryBreakdown, []DeckMastery, error) {
	// perDeck accumulates the disjoint learned/mature split per cardgroup while
	// the global breakdown accrues across every studied card.
	type deckAcc struct{ learned, mature int }
	perDeck := make(map[string]*deckAcc, len(deckCardTotals))
	mastery := MasteryBreakdown{TotalStudied: len(stats)}
	for _, s := range stats {
		tier := domain.ClassifyMastery(
			domain.FSRSState{Phase: s.Phase, Stability: s.Stability},
			domain.MatureStabilityDays,
		)
		acc := perDeck[s.CardgroupID]
		if acc == nil {
			acc = &deckAcc{}
			perDeck[s.CardgroupID] = acc
		}
		switch tier {
		case domain.TierInProgress:
			mastery.InProgress++
		case domain.TierLearned:
			mastery.Learned++
			acc.learned++
		case domain.TierMature:
			mastery.Mature++
			acc.mature++
		default:
			return MasteryBreakdown{}, nil, eris.Errorf("service: aggregate mastery: unhandled MasteryTier %d", tier)
		}
	}

	// Emit one DeckMastery for every owned deck (from deckCardTotals), so a deck
	// with zero studied cards still appears with learned=mature=0. Sort by
	// CardgroupID for a deterministic wire order.
	deckIDs := make([]string, 0, len(deckCardTotals))
	for id := range deckCardTotals {
		deckIDs = append(deckIDs, id)
	}
	sort.Strings(deckIDs)

	decks := make([]DeckMastery, 0, len(deckIDs))
	for _, id := range deckIDs {
		acc := perDeck[id]
		var learned, mature int
		if acc != nil {
			learned, mature = acc.learned, acc.mature
		}
		decks = append(decks, DeckMastery{
			CardgroupID:  id,
			TotalCards:   deckCardTotals[id],
			LearnedCards: learned,
			MatureCards:  mature,
		})
	}
	return mastery, decks, nil
}

// TopStruggling ranks studied FSRS stats by struggle: filter to cards with at
// least one lapse, sort by (Lapses desc, Stability asc), and cap at limit. This
// is the definition of "struggling" — a card with no lapse is never struggling,
// and among lapsed cards more lapses (then lower stability) rank higher. The
// result is always non-nil (an empty, non-nil slice when no card has lapsed).
func TopStruggling(stats []domain.FSRSStat, limit int) []StrugglingCard {
	out := make([]StrugglingCard, 0, len(stats))
	for _, s := range stats {
		if s.Lapses >= 1 {
			out = append(out, StrugglingCard{CardID: s.CardID, Lapses: s.Lapses, Stability: s.Stability})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Lapses != out[j].Lapses {
			return out[i].Lapses > out[j].Lapses // more lapses first
		}
		return out[i].Stability < out[j].Stability // ties: lower stability first
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// PerformanceMode is the difficulty level inferred from a user's recent
// performance. Values intentionally span ModeDifficult (0) .. ModeMastered (4).
type PerformanceMode int

const (
	ModeDifficult PerformanceMode = 0
	ModeDefault   PerformanceMode = 1
	ModeGood      PerformanceMode = 2
	ModeEasy      PerformanceMode = 3
	ModeMastered  PerformanceMode = 4

	MinReviewsForModeCalculation = 20

	// Success-rate band thresholds for ModeFromMetrics. A success rate at or
	// above each threshold selects the named mode (or higher).
	defaultModeThreshold  = 0.60
	goodModeThreshold     = 0.75
	easyModeThreshold     = 0.85
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

const (
	performanceWindowDays30 = 30
	performanceWindowDays7  = 7
)

// WindowedMetrics contains diagnostic snapshots for the trailing 365, 30, and
// 7-day windows. The caller supplies the already-loaded 365-day swipe set.
type WindowedMetrics struct {
	Days365 PerformanceMetrics
	Days30  PerformanceMetrics
	Days7   PerformanceMetrics
}

// ComputeWindowedMetrics computes all diagnostic windows from one already-loaded
// 365-day swipe set. Shorter windows include swipes exactly at their cutoff.
// StudyStreak is calendar truth rather than a window-scoped metric, so the 30-
// and 7-day snapshots always reuse the streak computed from the full set.
func ComputeWindowedMetrics(swipes []domain.SwipeRecord, now time.Time) WindowedMetrics {
	days30 := filterSwipesSince(swipes, now.AddDate(0, 0, -performanceWindowDays30))
	days7 := filterSwipesSince(swipes, now.AddDate(0, 0, -performanceWindowDays7))

	windows := WindowedMetrics{
		Days365: ComputeMetrics(swipes, now),
		Days30:  ComputeMetrics(days30, now),
		Days7:   ComputeMetrics(days7, now),
	}
	windows.Days30.StudyStreak = windows.Days365.StudyStreak
	windows.Days7.StudyStreak = windows.Days365.StudyStreak
	return windows
}

func filterSwipesSince(swipes []domain.SwipeRecord, cutoff time.Time) []domain.SwipeRecord {
	filtered := make([]domain.SwipeRecord, 0, len(swipes))
	for _, swipe := range swipes {
		if !swipe.ReviewedAt.Before(cutoff) {
			filtered = append(filtered, swipe)
		}
	}
	return filtered
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

// normalizedDifficulty maps a stored FSRS difficulty on the 1..10 scale into
// 0..1 as difficulty/10, clamped to [0,1]. The division is unconditional so a
// mastered card pinned at the FSRS floor of exactly 1.0 normalizes to 0.1 (the
// low-difficulty band) rather than 1.0; a strict `> 1` guard would leave the
// floor at 1.0 and trip the high-difficulty threshold, inverting the mode down.
func normalizedDifficulty(difficulty float64) float64 {
	difficulty = difficulty / 10
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
