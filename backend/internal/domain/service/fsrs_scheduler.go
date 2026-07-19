package service

import (
	"time"

	fsrs "github.com/open-spaced-repetition/go-fsrs/v3"

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
func (s *FSRSScheduler) Apply(state domain.FSRSState, rating domain.Rating, now time.Time) domain.FSRSState {
	info := s.algo.Next(fsrs.Card{
		Due:           state.Due,
		Stability:     state.Stability,
		Difficulty:    state.Difficulty,
		ElapsedDays:   uint64(state.ElapsedDays),
		ScheduledDays: uint64(state.ScheduledDays),
		Reps:          uint64(state.Reps),
		Lapses:        uint64(state.Lapses),
		State:         fsrs.State(state.Phase),
		LastReview:    state.LastReview,
	}, now, fsrs.Rating(rating))

	return domain.FSRSState{
		Due:           info.Card.Due,
		Stability:     info.Card.Stability,
		Difficulty:    info.Card.Difficulty,
		ElapsedDays:   int(info.Card.ElapsedDays),
		ScheduledDays: int(info.Card.ScheduledDays),
		Reps:          int(info.Card.Reps),
		Lapses:        int(info.Card.Lapses),
		Phase:         domain.FSRSPhase(info.Card.State),
		LastReview:    now,
		LastRating:    rating,
	}
}
