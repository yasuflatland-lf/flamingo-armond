package domain

import (
	"strings"
	"time"

	"github.com/rivo/uniseg"
	"github.com/rotisserie/eris"
)

const CardTextMax = 500

var (
	ErrCardCardgroupIDRequired = eris.New("card: cardgroup id is required")
	ErrCardFrontRequired       = eris.New("card: front is required")
	ErrCardFrontTooLong        = eris.Errorf("card: front exceeds %d characters", CardTextMax)
	ErrCardBackRequired        = eris.New("card: back is required")
	ErrCardBackTooLong         = eris.Errorf("card: back exceeds %d characters", CardTextMax)
)

// Card is an aggregate root. It references Cardgroup by ID only; GraphQL
// resolves the cross-aggregate object through a DataLoader.
type Card struct {
	ID          string
	CardgroupID string
	Front       string
	Back        string
	FSRS        FSRSState
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (c *Card) Validate() error {
	if strings.TrimSpace(c.CardgroupID) == "" {
		return ErrCardCardgroupIDRequired
	}
	if err := validateCardText("front", c.Front); err != nil {
		return err
	}
	if err := validateCardText("back", c.Back); err != nil {
		return err
	}
	return nil
}

func validateCardText(field, value string) error {
	n := uniseg.GraphemeClusterCount(strings.TrimSpace(value))
	switch {
	case n < 1 && field == "front":
		return ErrCardFrontRequired
	case n < 1:
		return ErrCardBackRequired
	case n > CardTextMax && field == "front":
		return ErrCardFrontTooLong
	case n > CardTextMax:
		return ErrCardBackTooLong
	default:
		return nil
	}
}
