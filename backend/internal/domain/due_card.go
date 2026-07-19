package domain

import "time"

// DueCard is a Card paired with the viewer's FSRS state for queueing decisions.
//
// Phase is FSRSPhaseNew when the viewer has no user_card_fsrs row for the card,
// in which case Due is the card's created_at as a stable substitute.
//
// DueCard is not an aggregate; it is a view-level value shared between the
// repository and OrderingPolicy. It lives in the domain package because the
// ordering policy is domain logic and DueCard is its input.
//
// Card must not be nil; downstream consumers (OrderingPolicy.Apply) dereference
// it unconditionally. The Phase invariant (FSRSPhaseNew ↔ Due == Card.CreatedAt)
// is established by the repository and is not enforced at the domain layer
// today; new construction sites must reproduce it.
type DueCard struct {
	Card  *Card
	Phase FSRSPhase
	Due   time.Time
	// Rescue is set by the repository for rows fetched by the rescue window;
	// OrderingPolicy must not move a non-rescue (filler) card ahead of a rescue card.
	Rescue bool
}
