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

// BelongsToMasterCardgroup mirrors Card.BelongsToCardgroup: it reports whether
// this master card belongs to the master cardgroup identified by
// masterCardgroupID. Empty masterCardgroupID always returns false so callers do
// not need a redundant nil/empty guard.
func (m *MasterCard) BelongsToMasterCardgroup(masterCardgroupID string) bool {
	return masterCardgroupID != "" && m.MasterCardgroupID == masterCardgroupID
}

// NewMasterCardFromValidated builds a MasterCard from parsed CardText values
// without re-validation and rejects their invalid zero value. It generates a
// fresh UUID v7 ID. CreatedAt and UpdatedAt are both stamped with now; batch
// callers pass one shared instant for the whole batch. Use NewMasterCard for
// untrusted strings that still need grapheme-bounds validation.
func NewMasterCardFromValidated(masterCardgroupID string, front, back CardText, position int, now time.Time) (*MasterCard, error) {
	if front == "" {
		return nil, ErrCardFrontRequired
	}
	if back == "" {
		return nil, ErrCardBackRequired
	}
	id, err := NewID()
	if err != nil {
		return nil, eris.Wrap(err, "master card: new id")
	}
	return &MasterCard{
		ID:                id,
		MasterCardgroupID: masterCardgroupID,
		Front:             front,
		Back:              back,
		Position:          position,
		CreatedAt:         now,
		UpdatedAt:         now,
	}, nil
}

// UpdateFront updates the master card's front text to the supplied value and
// returns an error if the value is the zero CardText. The zero-value guard is
// defense-in-depth: production callers parse the input through ParseCardText
// before reaching this method, so structural invariants (length, non-empty) are
// enforced at VO construction time. This method enforces only the aggregate-state
// invariant that m.Front must never be the zero value. UpdatedAt is intentionally
// not modified here; persistence is responsible for stamping the modification
// timestamp.
func (m *MasterCard) UpdateFront(front CardText) error {
	if front == "" {
		return ErrCardFrontRequired
	}
	m.Front = front
	return nil
}

// UpdateBack mirrors UpdateFront for the back text; see UpdateFront for the
// defense-in-depth rationale and the UpdatedAt non-stamping note.
func (m *MasterCard) UpdateBack(back CardText) error {
	if back == "" {
		return ErrCardBackRequired
	}
	m.Back = back
	return nil
}

// NewMasterCard constructs a MasterCard, validating and trimming Front and Back
// through ParseCardText and generating a fresh UUID v7 ID. CreatedAt and
// UpdatedAt are both stamped with now; batch callers pass one shared instant for
// the whole batch. It returns the same field-specific sentinels as NewCard.
func NewMasterCard(masterCardgroupID, front, back string, position int, now time.Time) (*MasterCard, error) {
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
