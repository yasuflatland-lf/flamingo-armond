package domain

import (
	"time"

	"github.com/rotisserie/eris"
)

// SwipeRecord is an immutable domain event. Append-only; never updated.
type SwipeRecord struct {
	ID          string
	UserID      string
	CardID      string
	CardgroupID string
	Rating      Rating
	ReviewedAt  time.Time
	StateAfter  FSRSState
}

// NewSwipeRecord creates a swipe event with a fresh UUID v7.
func NewSwipeRecord(userID, cardID, cardgroupID string, rating Rating, reviewedAt time.Time, stateAfter FSRSState) (*SwipeRecord, error) {
	if cardgroupID == "" {
		return nil, eris.New("swipe record: cardgroupID is required")
	}
	id, err := NewID()
	if err != nil {
		return nil, eris.Wrap(err, "swipe record: new id")
	}
	return &SwipeRecord{
		ID:          id,
		UserID:      userID,
		CardID:      cardID,
		CardgroupID: cardgroupID,
		Rating:      rating,
		ReviewedAt:  reviewedAt,
		StateAfter:  stateAfter,
	}, nil
}
