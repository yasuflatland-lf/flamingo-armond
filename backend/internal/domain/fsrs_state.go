package domain

import (
	"time"
)

// FSRSCardState mirrors go-fsrs card states without leaking that dependency
// into the domain package.
type FSRSCardState int

const (
	FSRSStateNew FSRSCardState = iota
	FSRSStateLearning
	FSRSStateReview
	FSRSStateRelearning
)

// IsValid reports whether the state is a recognised FSRSCardState constant.
func (s FSRSCardState) IsValid() bool {
	return s >= FSRSStateNew && s <= FSRSStateRelearning
}

// IsLearningPhase reports whether the card sits in a short-interval phase
// (Learning or Relearning) — i.e. its latest rating was Again or Hard. The
// learn queue's review slots prioritise these over long-interval Review cards.
func (s FSRSCardState) IsLearningPhase() bool {
	return s == FSRSStateLearning || s == FSRSStateRelearning
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
	State         FSRSCardState
	LastReview    time.Time
}

// NewFSRSStateForNewCard returns the initial scheduling state for a new card.
func NewFSRSStateForNewCard(now time.Time) FSRSState {
	return FSRSState{
		Due:        now,
		Stability:  2.5,
		Difficulty: 5.0,
		State:      FSRSStateNew,
		LastReview: now,
	}
}
