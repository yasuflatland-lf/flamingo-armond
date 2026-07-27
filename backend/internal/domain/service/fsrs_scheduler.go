package service

import (
	"fmt"
	"time"

	fsrs "github.com/open-spaced-repetition/go-fsrs/v4"

	"backend/internal/domain"
)

// FSRSScheduler wraps go-fsrs behind a pure domain service.
type FSRSScheduler struct{ algo *fsrs.FSRS }

func NewFSRSScheduler() *FSRSScheduler {
	params := fsrs.DefaultParam()
	// Long-term scheduling mode: skip the sub-day (minutes) learning steps so every
	// review is scheduled in whole-day intervals and cards never sit in the
	// Learning/Relearning phases. See go-fsrs Parameters.EnableShortTerm.
	params.EnableShortTerm = false
	return &FSRSScheduler{algo: fsrs.NewFSRS(params)}
}

// Apply returns a fresh state and does not mutate the input state.
//
// It panics when state.Phase is not a recognised FSRSPhase. The fsrs.State cast
// below feeds a library dispatch that has no default arm, so an unrecognised
// phase would produce a zero-valued scheduling result and silently wipe the
// card's state. An out-of-range Rating also panics because grades outside the
// domain constants are a programmer error. Panicking rather than returning an
// error keeps the single-value signature of the domain.FSRSScheduler consumer
// interface: no caller can build an invalid phase — Phase is only ever set by
// this method or reconstituted by the repository, whose read guard already
// rejects an unrecognised persisted value — so these are programmer-error
// guards of the same kind as the constructor panic in
// domain.NewUserCardFSRSForNewCard.
//
// An error from the v4 Next call also panics. The rating, phase, difficulty,
// and clock checks above or at repository load make the corresponding input
// errors unreachable for validated state. A stability underflow or invalid
// library result has no caller-side remedy, and accepting its zero-valued
// scheduling result would silently destroy the card's state.
//
// ElapsedDays on the returned state is computed by
// domain.FSRSState.ElapsedDaysAt, not read from the scheduler library.
func (s *FSRSScheduler) Apply(state domain.FSRSState, rating domain.Rating, now time.Time) domain.FSRSState {
	if !state.Phase.IsValid() {
		panic(fmt.Sprintf("service: fsrs scheduler: invalid FSRSPhase %d", int(state.Phase)))
	}
	if !rating.IsValid() {
		panic(fmt.Sprintf("service: fsrs scheduler: invalid Rating %d", int(rating)))
	}

	// A backward clock step (NTP, cross-instance skew) would make the library's
	// elapsed-days float negative; the float->uint64 conversion of a negative
	// value is implementation-dependent (Go spec, Conversions) and corrupts the
	// persisted state on amd64. Clamp so elapsed time can never be negative.
	if now.Before(state.LastReview) {
		now = state.LastReview
	}

	info, err := s.algo.Next(fsrs.Card{
		Due:           state.Due,
		Stability:     state.Stability,
		Difficulty:    state.Difficulty,
		ScheduledDays: uint64(state.ScheduledDays),
		Reps:          uint64(state.Reps),
		Lapses:        uint64(state.Lapses),
		State:         fsrs.State(state.Phase),
		LastReview:    state.LastReview,
	}, now, fsrs.Rating(rating))
	if err != nil {
		panic(fmt.Sprintf("service: fsrs scheduler: v4 rejected a validated input: %v", err))
	}

	return domain.FSRSState{
		Due:           info.Card.Due,
		Stability:     info.Card.Stability,
		Difficulty:    info.Card.Difficulty,
		ElapsedDays:   state.ElapsedDaysAt(now),
		ScheduledDays: int(info.Card.ScheduledDays),
		Reps:          int(info.Card.Reps),
		Lapses:        int(info.Card.Lapses),
		Phase:         domain.FSRSPhase(info.Card.State),
		LastReview:    now,
		LastRating:    rating,
	}
}
