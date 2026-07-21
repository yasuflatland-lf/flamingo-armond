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

// NewMasterCardFromValidated mirrors NewCardFromValidated for the master
// aggregate: it builds a MasterCard from already-parsed CardText value objects
// and skips the ParseCardText re-validation NewMasterCard would perform. Callers
// must pass VOs produced by a prior Parse; the zero-value CardText is rejected
// via the field-specific sentinel because the newtype's zero value is invalid by
// contract. A fresh UUID v7 ID is generated and CreatedAt/UpdatedAt are stamped
// with the current UTC time; batch callers may override both with a shared
// timestamp before persisting. Use NewMasterCard for the untrusted string-input
// path that still needs grapheme-bounds validation.
func NewMasterCardFromValidated(masterCardgroupID string, front, back CardText, position int) (*MasterCard, error) {
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
	now := time.Now().UTC()
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
