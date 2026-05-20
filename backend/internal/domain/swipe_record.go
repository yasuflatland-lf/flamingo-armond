package domain

import (
	"time"

	"github.com/google/uuid"
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
	id, err := uuid.NewV7()
	if err != nil {
		return nil, eris.Wrap(err, "swipe record: new uuid v7")
	}
	return &SwipeRecord{
		ID:          id.String(),
		UserID:      userID,
		CardID:      cardID,
		CardgroupID: cardgroupID,
		Rating:      rating,
		ReviewedAt:  reviewedAt,
		StateAfter:  stateAfter,
	}, nil
}
