package domain

import (
	"time"

	"github.com/rotisserie/eris"
)

// CardgroupNameMax is the maximum number of grapheme clusters allowed in a
// cardgroup name. Exported so the usecase layer can surface the limit in error
// messages without duplicating the constant.
const CardgroupNameMax = 100

// Sentinel errors for cardgroup name validation. Callers should use
// errors.Is to match them rather than comparing message strings.
var (
	ErrCardgroupNameRequired = eris.New("cardgroup: name is required")
	ErrCardgroupNameTooLong  = eris.Errorf("cardgroup: name exceeds %d characters", CardgroupNameMax)
)

// Cardgroup is an aggregate root: a named collection of cards owned by exactly
// one User. The collection of Cards is a separate aggregate; Cardgroup holds
// only its OwnerID, never an embedded *User.
type Cardgroup struct {
	ID        string
	OwnerID   string
	Name      CardgroupName
	CreatedAt time.Time
	UpdatedAt time.Time
}

// IsOwnedBy reports whether the cardgroup belongs to the user identified by userID.
// Empty userID always returns false so callers do not need a redundant nil/empty guard.
func (c Cardgroup) IsOwnedBy(userID string) bool {
	return userID != "" && c.OwnerID == userID
}

// Rename updates the cardgroup's name. The non-empty invariant is enforced at
// VO construction time by ParseCardgroupName; this method's zero-value guard
// is a safety net for callers that construct CardgroupName directly (via the
// string cast) and bypass the parser. Length is NOT re-checked here — that
// invariant lives only in ParseCardgroupName. UpdatedAt is intentionally not
// modified; persistence is responsible for stamping the modification timestamp.
func (c *Cardgroup) Rename(name CardgroupName) error {
	if name == "" {
		return ErrCardgroupNameRequired
	}
	c.Name = name
	return nil
}
