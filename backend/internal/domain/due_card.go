package domain

import "time"

// DueCard is a Card paired with the viewer's FSRS state for queueing decisions.
//
// A card is unseen exactly when no user_card_fsrs row exists for the (user,
// card) pair. The repository synthesizes such a row as Phase = FSRSPhaseNew and
// Due = Card.CreatedAt, a stable display placeholder that keeps the read-model
// orderable. The implication runs one way only: an unseen card always carries
// Due == Card.CreatedAt, but a reviewed card whose scheduled Due happens to land
// on its CreatedAt satisfies the same equality. Test Phase == FSRSPhaseNew (or
// the FSRS row itself) for unseen; never the Due/CreatedAt equality.
//
// DueCard is not an aggregate; it is a view-level value shared between the
// repository and OrderingPolicy. It lives in the domain package because the
// ordering policy is domain logic and DueCard is its input.
//
// Card must not be nil; downstream consumers (OrderingPolicy.Apply) dereference
// it unconditionally. The Phase/Due synthesis is established by the repository
// and is not enforced at the domain layer today; new construction sites must
// reproduce it.
type DueCard struct {
	Card  *Card
	Phase FSRSPhase
	Due   time.Time
	// Rescue is set by the repository for rows fetched by the rescue window;
	// OrderingPolicy must not move a non-rescue (filler) card ahead of a rescue card.
	Rescue bool
}
