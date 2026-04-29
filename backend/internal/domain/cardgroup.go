package domain

import (
	"strings"
	"time"

	"github.com/rivo/uniseg"
	"github.com/rotisserie/eris"
)

const (
	cardgroupNameMin = 1
	cardgroupNameMax = 100
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
// Name (after trimming surrounding whitespace) must be 1-100 grapheme clusters.
func (c *Cardgroup) Validate() error {
	name := strings.TrimSpace(c.Name)
	n := uniseg.GraphemeClusterCount(name)
	if n < cardgroupNameMin {
		return eris.New("cardgroup: name is required")
	}
	if n > cardgroupNameMax {
		return eris.Errorf("cardgroup: name must be at most %d characters", cardgroupNameMax)
	}
	return nil
}
