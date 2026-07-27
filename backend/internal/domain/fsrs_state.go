package domain

import (
	"math"
	"time"
)

// FSRSPhase mirrors go-fsrs card states without leaking that dependency
// into the domain package.
type FSRSPhase int

// Learning and Relearning occur only on legacy rows written before the
// long-term scheduler switch. Their next swipe absorbs them into Review.
const (
	FSRSPhaseNew FSRSPhase = iota
	FSRSPhaseLearning
	FSRSPhaseReview
	FSRSPhaseRelearning
)

// IsValid reports whether the phase is a recognised FSRSPhase constant.
func (p FSRSPhase) IsValid() bool {
	return p >= FSRSPhaseNew && p <= FSRSPhaseRelearning
}

// MinDifficulty and MaxDifficulty bound the FSRS difficulty scale. go-fsrs
// clamps every difficulty it produces into this closed range, and
// NewCardDifficulty sits mid-scale, so no difficulty the application persists
// can fall outside it.
const (
	MinDifficulty = 1.0
	MaxDifficulty = 10.0
)

// IsValidStability reports whether s is a stability the scheduler can produce:
// finite and strictly positive. The check exists because NaN and the infinities
// fail every ordered comparison silently — an unchecked NaN stability falls
// through both ClassifyMastery comparisons and is reported as the Learned tier,
// and it breaks JSON marshalling of the GraphQL Float it feeds.
func IsValidStability(s float64) bool {
	if math.IsNaN(s) || math.IsInf(s, 0) {
		return false
	}
	return s > 0
}

// IsValidDifficulty reports whether d sits within [MinDifficulty, MaxDifficulty].
// NaN and the infinities fail the comparison and are therefore rejected too.
func IsValidDifficulty(d float64) bool {
	return d >= MinDifficulty && d <= MaxDifficulty
}

// FSRSState is an immutable value object. Repository code persists it as a
// flat column block, but domain consumers treat it as one scheduling state.
type FSRSState struct {
	Due           time.Time
	Stability     float64
	Difficulty    float64
	ElapsedDays   int
	ScheduledDays int
	Reps          int
	Lapses        int
	Phase         FSRSPhase
	LastReview    time.Time
	// LastRating is the rating of the swipe that produced this state; the zero
	// value denotes a synthesized new-card state that no swipe has rated yet.
	LastRating Rating
}

// ElapsedDaysAt returns the whole days elapsed since LastReview at now — the
// value FSRSState.ElapsedDays carries. This is the application's definition, not
// the scheduler library's: go-fsrs v4 removes Card.ElapsedDays and counts
// elapsed days internally by UTC calendar date without exposing the result.
// domain.rescueMinElapsed and service.isOnTimeRecall both read against this
// definition, so changing the formula moves the 24h rescue floor and the
// on-time-recall statistic together.
//
// A New card and a card with no LastReview both yield 0: neither has a prior
// review to measure from. The now.Before branch is a clock-skew guard — Apply
// already clamps now to LastReview before calling this, but the method must be
// correct on its own.
func (s FSRSState) ElapsedDaysAt(now time.Time) int {
	if s.Phase == FSRSPhaseNew || s.LastReview.IsZero() || now.Before(s.LastReview) {
		return 0
	}
	return int(now.Sub(s.LastReview).Hours() / 24)
}

// NewCardStability and NewCardDifficulty are the placeholder scheduling values a
// card carries until its first review. The scheduler overwrites both on that
// first review, so neither ever influences a computed interval: they exist so a
// never-reviewed card has a displayable, in-range state, and so the first swipe
// records a non-zero StabilityBefore snapshot (which the statistics known-review
// gate reads).
const (
	NewCardStability  = 2.5
	NewCardDifficulty = 5.0
)

// NewFSRSStateForNewCard returns the initial scheduling state for a new card.
func NewFSRSStateForNewCard(now time.Time) FSRSState {
	return FSRSState{
		Due:        now,
		Stability:  NewCardStability,
		Difficulty: NewCardDifficulty,
		Phase:      FSRSPhaseNew,
		LastReview: now,
	}
}
