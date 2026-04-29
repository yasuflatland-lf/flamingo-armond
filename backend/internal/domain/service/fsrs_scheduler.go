package service

import (
	"time"

	fsrs "github.com/open-spaced-repetition/go-fsrs/v3"

	"backend/internal/domain"
)

// FSRSScheduler wraps go-fsrs behind a pure domain service.
type FSRSScheduler struct{ algo *fsrs.FSRS }

func NewFSRSScheduler() *FSRSScheduler {
	return &FSRSScheduler{algo: fsrs.NewFSRS(fsrs.DefaultParam())}
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
		State:         fsrs.State(state.State),
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
		State:         domain.FSRSCardState(info.Card.State),
		LastReview:    now,
	}
}
