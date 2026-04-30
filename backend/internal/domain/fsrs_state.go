package domain

import (
	"errors"
	"time"
)

var (
	ErrFSRSOverridePartial      = errors.New("fsrs override must specify all nine fields or none")
	ErrFSRSOverrideStateInvalid = errors.New("fsrs override state out of range")
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

// FSRSStateOverride carries optional GraphQL input pointers for overriding the
// default new-card FSRS state. All nine fields must be non-nil together, or all
// must be nil; a mix is rejected by NewFSRSStateFromInput.
type FSRSStateOverride struct {
	Due           *time.Time
	Stability     *float64
	Difficulty    *float64
	ElapsedDays   *int
	ScheduledDays *int
	Reps          *int
	Lapses        *int
	State         *int // raw int from GraphQL; cast to FSRSCardState (0–3)
	LastReview    *time.Time
}

// NewFSRSStateFromInput resolves an FSRSStateOverride into a concrete FSRSState.
//   - All nine pointers nil  -> returns NewFSRSStateForNewCard(now), nil.
//   - All nine pointers non-nil -> returns the override values, nil.
//   - Mixed nil + non-nil   -> returns zero value and ErrFSRSOverridePartial.
//   - State out of 0..3     -> returns zero value and ErrFSRSOverrideStateInvalid.
func NewFSRSStateFromInput(in FSRSStateOverride, now time.Time) (FSRSState, error) {
	nilCount := 0
	if in.Due == nil {
		nilCount++
	}
	if in.Stability == nil {
		nilCount++
	}
	if in.Difficulty == nil {
		nilCount++
	}
	if in.ElapsedDays == nil {
		nilCount++
	}
	if in.ScheduledDays == nil {
		nilCount++
	}
	if in.Reps == nil {
		nilCount++
	}
	if in.Lapses == nil {
		nilCount++
	}
	if in.State == nil {
		nilCount++
	}
	if in.LastReview == nil {
		nilCount++
	}

	const total = 9

	switch nilCount {
	case total:
		// All nil: return the default new-card state.
		return NewFSRSStateForNewCard(now), nil
	case 0:
		// All non-nil: validate State range then build the value object.
		if *in.State < int(FSRSStateNew) || *in.State > int(FSRSStateRelearning) {
			return FSRSState{}, ErrFSRSOverrideStateInvalid
		}
		return FSRSState{
			Due:           *in.Due,
			Stability:     *in.Stability,
			Difficulty:    *in.Difficulty,
			ElapsedDays:   *in.ElapsedDays,
			ScheduledDays: *in.ScheduledDays,
			Reps:          *in.Reps,
			Lapses:        *in.Lapses,
			State:         FSRSCardState(*in.State),
			LastReview:    *in.LastReview,
		}, nil
	default:
		// Partial override: reject.
		return FSRSState{}, ErrFSRSOverridePartial
	}
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
