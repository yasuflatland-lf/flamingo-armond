package service

import (
	"fmt"
	"time"

	fsrs "github.com/open-spaced-repetition/go-fsrs/v4"

	"backend/internal/domain"
)

// FSRSScheduler wraps go-fsrs behind a pure domain service.
type FSRSScheduler struct{ algo *fsrs.FSRS }

// NewFSRSScheduler builds the long-term-mode scheduler the whole domain assumes:
// whole-day intervals only, and no card ever sitting in Learning or Relearning.
//
// Both guards below exist because fsrs.NewFSRS substitutes fsrs.DefaultParam()
// for the caller's whole Parameters value when validation fails, and DefaultParam
// sets EnableShortTerm = true. One out-of-range field would therefore swap in the
// short-term scheduler with no error and no failing test: sub-day intervals would
// return and domain.LearnedStabilityDays' mastery/statistics parity would break at
// a site that has no compile-time link to this one.
//
// LearningSteps and RelearningSteps are cleared because the long-term scheduler
// never reads them. They are not inert: clipParameters derives the W17/W18 ceiling
// from len(RelearningSteps). That ceiling is 2.0 for any length below 2, which is
// what both the default one-element slice and nil produce, so clearing them moves
// no golden value — it removes the coupling.
func NewFSRSScheduler() *FSRSScheduler {
	params := fsrs.DefaultParam()
	params.EnableShortTerm = false
	params.LearningSteps = nil
	params.RelearningSteps = nil
	if err := params.Validate(); err != nil {
		panic(fmt.Sprintf("service: fsrs scheduler: invalid parameters: %v", err))
	}
	algo := fsrs.NewFSRS(params)
	if algo.EnableShortTerm {
		panic("service: fsrs scheduler: go-fsrs discarded EnableShortTerm = false")
	}
	return &FSRSScheduler{algo: algo}
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
// fsrs.Card.RemainingSteps is deliberately not carried on FSRSState. The
// long-term scheduler never reads it and setReviewState zeroes it on every
// output, so a round trip through the domain is lossless. That holds only while
// EnableShortTerm is false, which the constructor above now asserts — under
// short-term mode the basic scheduler reads it to advance the learning step, and
// every card would restart at step 0.
func (s *FSRSScheduler) Apply(state domain.FSRSState, rating domain.Rating, now time.Time) domain.FSRSState {
	if !state.Phase.IsValid() {
		panic(fmt.Sprintf("service: fsrs scheduler: invalid FSRSPhase %d", int(state.Phase)))
	}
	if !rating.IsValid() {
		panic(fmt.Sprintf("service: fsrs scheduler: invalid Rating %d", int(rating)))
	}

	// A backward clock step (NTP, cross-instance skew) leaves LastReview after
	// now, which the library's card validation rejects — and the error arm below
	// panics, so an unclamped skew would turn a routine review into an INTERNAL
	// error. Clamp so elapsed time can never be negative.
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
