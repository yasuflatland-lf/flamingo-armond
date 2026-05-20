package domain

import "time"

// DueCard is a Card paired with the viewer's FSRS state for queueing decisions.
//
// State is FSRSStateNew when the viewer has no user_card_fsrs row for the card,
// in which case Due is the card's created_at as a stable substitute.
//
// DueCard is not an aggregate; it is a view-level value shared between the
// repository and OrderingPolicy. It lives in the domain package because the
// ordering policy is domain logic and DueCard is its input.
type DueCard struct {
	Card  *Card
	State FSRSCardState
	Due   time.Time
}
