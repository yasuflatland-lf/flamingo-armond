package domain

import (
	"time"

	"github.com/rotisserie/eris"
)

// MasterCard is a single card within a MasterCardgroup. It references its
// parent by MasterCardgroupID only; cross-aggregate resolution is handled by
// the repository layer.
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

// NewMasterCard constructs a MasterCard aggregate, mirroring NewCard: Front and
// Back are validated and trimmed through ParseCardText with the same
// field-specific sentinels (ErrCardFrontRequired / ErrCardFrontTooLong for
// Front, ErrCardBackRequired / ErrCardBackTooLong for Back), and a fresh UUID
// v7 ID is generated. CreatedAt and UpdatedAt are stamped with the current UTC
// time; batch callers may override both with a shared timestamp before
// persisting.
func NewMasterCard(masterCardgroupID, front, back string, position int) (*MasterCard, error) {
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
		return nil, eris.Wrap(err, "master card: new id")
	}
	now := time.Now().UTC()
	return &MasterCard{
		ID:                id,
		MasterCardgroupID: masterCardgroupID,
		Front:             frontVO,
		Back:              backVO,
		Position:          position,
		CreatedAt:         now,
		UpdatedAt:         now,
	}, nil
}
