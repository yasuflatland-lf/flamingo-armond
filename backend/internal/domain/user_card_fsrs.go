package domain

import "time"

// UserCardFSRS is the per-user scheduling aggregate for a card.
type UserCardFSRS struct {
	UserID    string
	CardID    string
	State     FSRSState
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewUserCardFSRSForNewCard(userID, cardID string, now time.Time) *UserCardFSRS {
	return &UserCardFSRS{
		UserID:    userID,
		CardID:    cardID,
		State:     NewFSRSStateForNewCard(now),
		CreatedAt: now,
		UpdatedAt: now,
	}
}
