package domain

import (
	"time"
)

// FSRSPhase mirrors go-fsrs card states without leaking that dependency
// into the domain package.
type FSRSPhase int

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

// IsLearningPhase reports whether the card sits in a short-interval phase
// (Learning or Relearning) — i.e. its latest rating was Again or Hard. The
// learn queue's review slots prioritise these over long-interval Review cards.
func (p FSRSPhase) IsLearningPhase() bool {
	return p == FSRSPhaseLearning || p == FSRSPhaseRelearning
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
