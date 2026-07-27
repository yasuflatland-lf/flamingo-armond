package domain

import (
	"time"

	"github.com/rotisserie/eris"
)

// UserCardFSRS is the per-user scheduling aggregate for a card.
type UserCardFSRS struct {
	UserID    UserID
	CardID    string
	State     FSRSState
	CreatedAt time.Time
	UpdatedAt time.Time
}

// FSRSScheduler is the consumer-defined interface for the FSRS scheduler.
// *service.FSRSScheduler satisfies it implicitly.
type FSRSScheduler interface {
	Apply(state FSRSState, rating Rating, now time.Time) FSRSState
}

func NewUserCardFSRSForNewCard(userID UserID, cardID string, now time.Time) *UserCardFSRS {
	if userID == "" || cardID == "" {
		panic("user_card_fsrs: userID and cardID must not be empty")
	}
	return &UserCardFSRS{
		UserID:    userID,
		CardID:    cardID,
		State:     NewFSRSStateForNewCard(now),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// UserCardFSRSOrNew returns ucs when a scheduling record exists and the default
// new-card state otherwise. A nil record is the DataLoader's documented
// "never seen this card" signal; keeping this reconstitution rule on the
// aggregate prevents the resolver read path and swipe write path from drifting
// on what a missing row means.
func UserCardFSRSOrNew(ucs *UserCardFSRS, userID UserID, cardID string, createdAt time.Time) *UserCardFSRS {
	if ucs != nil {
		return ucs
	}
	return NewUserCardFSRSForNewCard(userID, cardID, createdAt)
}

// ApplyRating recomputes the scheduling state via the provided scheduler and
// stamps UpdatedAt with now. An invalid rating leaves the aggregate
// unchanged and returns an error.
func (u *UserCardFSRS) ApplyRating(scheduler FSRSScheduler, rating Rating, now time.Time) error {
	if !rating.IsValid() {
		return eris.Errorf("user_card_fsrs: invalid rating %d", rating)
	}
	u.State = scheduler.Apply(u.State, rating, now)
	u.UpdatedAt = now
	return nil
}
