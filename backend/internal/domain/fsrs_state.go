package domain

import (
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

// NewFSRSStateForNewCard returns the initial scheduling state for a new card.
func NewFSRSStateForNewCard(now time.Time) FSRSState {
	return FSRSState{
		Due:        now,
		Stability:  2.5,
		Difficulty: 5.0,
		Phase:      FSRSPhaseNew,
		LastReview: now,
	}
}
