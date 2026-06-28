package domain

import (
	"time"

	"github.com/rotisserie/eris"
)

const CardTextMax = 500

var (
	ErrCardFrontRequired = eris.New("card: front is required")
	ErrCardFrontTooLong  = eris.Errorf("card: front exceeds %d characters", CardTextMax)
	ErrCardBackRequired  = eris.New("card: back is required")
	ErrCardBackTooLong   = eris.Errorf("card: back exceeds %d characters", CardTextMax)
)

// Card is an aggregate root. It references Cardgroup by ID only; GraphQL
// resolves the cross-aggregate object through a DataLoader.
//
// CardgroupID ownership is enforced at the usecase boundary via
// authorizeCardgroupOrBadInput before a Card is constructed; the domain
// aggregate therefore does not re-check CardgroupID presence in NewCard.
type Card struct {
	ID          string
	CardgroupID CardgroupID
	Front       CardText
	Back        CardText
	CreatedAt   time.Time
	UpdatedAt   time.Time
	// Position persists the card's place within its cardgroup's source
	// document (Notion sync order). It defaults to 0 for cards not created
	// via Notion sync. The learn-session OrderingPolicy does not consult
	// Position: due-card ordering is driven by the discovery-first policy in
	// domain/service, whose DueCard view does not carry this field.
	Position int
}

// BelongsToCardgroup reports whether this card belongs to the cardgroup identified by
// cardgroupID. Empty cardgroupID always returns false so callers do not need a redundant
// nil/empty guard.
func (c *Card) BelongsToCardgroup(cardgroupID CardgroupID) bool {
	return cardgroupID != "" && c.CardgroupID == cardgroupID
}

// NewCard constructs a Card aggregate, enforcing its invariants at construction
// time: Front and Back are validated and trimmed through ParseCardText and a
// fresh UUID v7 ID is generated. CreatedAt and UpdatedAt are stamped with the
// current UTC time; batch callers may override both with a shared timestamp
// before persisting. Returns the field-specific CardText sentinel (e.g.
// ErrCardFrontRequired) on invalid input — callers translate it via
// translateCardErr — or a wrapped error when ID generation fails.
func NewCard(cardgroupID CardgroupID, front, back string, position int) (*Card, error) {
	frontVO, err := ParseCardText(front, ErrCardFrontRequired, ErrCardFrontTooLong)
	if err != nil {
		return nil, err
	}
	backVO, err := ParseCardText(back, ErrCardBackRequired, ErrCardBackTooLong)
	if err != nil {
		return nil, err
	}
	id, err := NewID()
	if err != nil {
		return nil, eris.Wrap(err, "card: new id")
	}
	now := time.Now().UTC()
	return &Card{
		ID:          id,
		CardgroupID: cardgroupID,
		Front:       frontVO,
		Back:        backVO,
		Position:    position,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// UpdateFront updates the card's front text to the supplied value and returns an
// error if the value is the zero CardText. The zero-value guard is defense-in-depth:
// production callers parse the input through ParseCardText before reaching this
// method, so structural invariants (length, non-empty) are enforced at VO construction
// time. This method enforces only the aggregate-state invariant that c.Front must
// never be the zero value. UpdatedAt is intentionally not modified here; persistence
// is responsible for stamping the modification timestamp.
func (c *Card) UpdateFront(front CardText) error {
	if front == "" {
		return ErrCardFrontRequired
	}
	c.Front = front
	return nil
}

// UpdateBack mirrors UpdateFront for the back text; see UpdateFront for the
// defense-in-depth rationale and the UpdatedAt non-stamping note.
func (c *Card) UpdateBack(back CardText) error {
	if back == "" {
		return ErrCardBackRequired
	}
	c.Back = back
	return nil
}
