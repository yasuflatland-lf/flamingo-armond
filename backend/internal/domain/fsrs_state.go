package domain

import "time"

// FSRSCardState mirrors go-fsrs card states without leaking that dependency
// into the domain package.
type FSRSCardState int

const (
	FSRSStateNew FSRSCardState = iota
	FSRSStateLearning
	FSRSStateReview
	FSRSStateRelearning
)

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
