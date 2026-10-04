package domain

import (
	"time"

	"github.com/rotisserie/eris"
)

// SwipeRecord is an immutable domain event. Append-only; never updated.
type SwipeRecord struct {
	ID              string
	UserID          UserID
	CardID          string
	CardgroupID     CardgroupID
	Rating          Rating
	ReviewedAt      time.Time
	DifficultyAfter float64
	// PhaseBefore, StabilityBefore and DueBefore are the pre-swipe snapshot: the
	// phase and stability the card held going in, and the due instant the review is
	// judged against. All three are NOT NULL columns -- the nullable era ended with
	// the swipe history that needed the sentinel.
	PhaseBefore     FSRSPhase
	StabilityBefore float64
	DueBefore       time.Time
}

// NewSwipeRecord creates a swipe event with a fresh UUID v7. The pre-swipe snapshot
// comes from stateBefore, the state just before the rating was applied, so the metrics
// layer can tell a Learning->Easy graduation from a review of an already-learned card.
// difficultyAfter is the difficulty the swipe advanced the card to; it feeds the /stats
// average and is the only post-swipe value the log keeps.
func NewSwipeRecord(userID UserID, cardID string, cardgroupID CardgroupID, rating Rating, reviewedAt time.Time, stateBefore FSRSState, difficultyAfter float64) (*SwipeRecord, error) {
	if cardgroupID == "" {
		return nil, eris.New("swipe record: cardgroupID is required")
	}
	id, err := NewID()
	if err != nil {
		return nil, eris.Wrap(err, "swipe record: new id")
	}
	return &SwipeRecord{
		ID:              id,
		UserID:          userID,
		CardID:          cardID,
		CardgroupID:     cardgroupID,
		Rating:          rating,
		ReviewedAt:      reviewedAt,
		DifficultyAfter: difficultyAfter,
		PhaseBefore:     stateBefore.Phase,
		StabilityBefore: stateBefore.Stability,
		DueBefore:       stateBefore.Due,
	}, nil
}
