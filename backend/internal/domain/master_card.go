package domain

import "time"

// MasterCard is a single card within a MasterCardgroup. It references its
// parent by MasterCardgroupID only; cross-aggregate resolution is handled by
// the repository and DataLoader layers.
//
// Front and Back are CardText values so the same grapheme-cluster length
// invariant (1..CardTextMax) and sentinel reuse apply without duplicating
// validation logic.
type MasterCard struct {
	ID                string
	MasterCardgroupID string
	Front             CardText
	Back              CardText
	Position          int
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Validate checks that Front and Back are non-empty and within the CardTextMax
// limit. It reuses ParseCardText with the same field-specific sentinels as
// Card.Validate() — ErrCardFrontRequired / ErrCardFrontTooLong for Front, and
// ErrCardBackRequired / ErrCardBackTooLong for Back.
func (c *MasterCard) Validate() error {
	if _, err := ParseCardText(string(c.Front), ErrCardFrontRequired, ErrCardFrontTooLong); err != nil {
		return err
	}
	if _, err := ParseCardText(string(c.Back), ErrCardBackRequired, ErrCardBackTooLong); err != nil {
		return err
	}
	return nil
}
