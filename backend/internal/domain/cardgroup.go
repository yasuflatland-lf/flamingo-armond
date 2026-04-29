package domain

import (
	"strings"
	"time"

	"github.com/rivo/uniseg"
	"github.com/rotisserie/eris"
)

// CardgroupNameMax is the maximum number of grapheme clusters allowed in a
// cardgroup name. Exported so the usecase layer can surface the limit in error
// messages without duplicating the constant.
const CardgroupNameMax = 100

// Sentinel errors returned by Cardgroup.Validate. Callers should use
// errors.Is to match them rather than comparing message strings.
var (
	ErrCardgroupNameRequired = eris.New("cardgroup: name is required")
	ErrCardgroupNameTooLong  = eris.New("cardgroup: name exceeds 100 characters")
)

// Cardgroup is an aggregate root: a named collection of cards owned by exactly
// one User. The collection of Cards is a separate aggregate; Cardgroup holds
// only its OwnerID, never an embedded *User.
type Cardgroup struct {
	ID        string
	OwnerID   string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Validate enforces invariants on the cardgroup aggregate. The Postgres CHECK
// constraint is a coarse floor (UTF-8 code points); this method is the
// authoritative bound (grapheme clusters).
//
// Name (after trimming surrounding whitespace) must be 1-CardgroupNameMax
// grapheme clusters. Returns ErrCardgroupNameRequired or ErrCardgroupNameTooLong
// on violation so callers can match with errors.Is.
func (c *Cardgroup) Validate() error {
	name := strings.TrimSpace(c.Name)
	n := uniseg.GraphemeClusterCount(name)
	if n < 1 {
		return ErrCardgroupNameRequired
	}
	if n > CardgroupNameMax {
		return ErrCardgroupNameTooLong
	}
	return nil
}
