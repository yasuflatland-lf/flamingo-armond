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
// aggregate therefore does not re-check CardgroupID presence in Validate.
type Card struct {
	ID          string
	CardgroupID string
	Front       CardText
	Back        CardText
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (c *Card) Validate() error {
	if _, err := ParseCardText(string(c.Front), ErrCardFrontRequired, ErrCardFrontTooLong); err != nil {
		return err
	}
	if _, err := ParseCardText(string(c.Back), ErrCardBackRequired, ErrCardBackTooLong); err != nil {
		return err
	}
	return nil
}
