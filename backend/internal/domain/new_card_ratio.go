package domain

import (
	"errors"
	"math/big"

	"github.com/rotisserie/eris"
)

// NewCardRatio is the per-user new-vs-review interleave ratio for a learn
// session, held as an irreducible fraction num/den. num is the new-card share
// (interleave nRatio); den is the total, so the review share is den-num
// (interleave rRatio). The zero value is invalid; construct via
// ParseNewCardRatio or use DefaultNewCardRatio.
type NewCardRatio struct {
	num int // new-card share; invariant 1 <= num < den
	den int // total; invariant den <= NewCardRatioDenMax, gcd(num, den) == 1
}

// DefaultLearnSessionSize is the number of cards served when the caller does
// not request a size. It bounds NewCardRatioDenMax because interleave emits
// whole cycles and learn truncates them; a larger denominator allows the
// leading review run to fill the page and collapse the new-card share to zero.
// usecase.defaultLearnNextDueLimit uses this constant to prevent drift.
const DefaultLearnSessionSize = 20

// NewCardRatioDenMax caps the reduced denominator at the default learn-session
// size so every stored ratio is exactly representable in a default session.
const NewCardRatioDenMax = DefaultLearnSessionSize

// NewCardRatioMaxNewShareNum / NewCardRatioMaxNewShareDen cap the new-card
// share at 4/5 (80%) so the review share stays >= 20% — the discovery-first
// floor. Above it review slots starve and the backlog grows unbounded; proven in
// docs/backend/ddd-patterns/discovery-first-due-ordering.md and the formal study.
const (
	NewCardRatioMaxNewShareNum = 4
	NewCardRatioMaxNewShareDen = 5
)

// ParseNewCardRatio's rejection reasons, one sentinel per rule so callers can
// attribute the fault to a field with errors.Is instead of re-deriving the
// bounds. The bound-free reasons use plain errors.New so errors.Is matches by
// identity rather than by eris's message equality; the bound-carrying reasons
// follow the sibling precedent (ErrRoleNameTooLong) and interpolate their
// exported bounds so the constants and messages cannot drift apart.
var (
	ErrNewCardRatioDenominatorNotPositive = errors.New("domain: new card ratio: denominator must be positive")
	ErrNewCardRatioShareOutOfRange        = errors.New("domain: new card ratio: numerator must satisfy 0 < num < den")
	ErrNewCardRatioDenominatorTooLarge    = eris.Errorf("domain: new card ratio: reduced denominator exceeds max %d", NewCardRatioDenMax)
	ErrNewCardRatioNewShareTooHigh        = eris.Errorf(
		"domain: new card ratio: new share must not exceed %d/%d",
		NewCardRatioMaxNewShareNum, NewCardRatioMaxNewShareDen)
)

// DefaultNewCardRatio (4/5) is applied when a user has no stored preference.
// New share 4, review share 1 → interleave 4:1, matching the historical
// hard-coded 4:1 new:review interleave this VO replaces.
var DefaultNewCardRatio = mustNewCardRatio(4, 5)

// ParseNewCardRatio reduces num/den and validates each bound in check order.
// ErrNewCardRatioDenominatorTooLarge means the reduced denominator exceeds the
// default session size; interleave emits whole cycles while learn serves only
// a session-sized prefix, so such a ratio is not representable. Other sentinels
// cover a non-positive denominator, an invalid share, or a share above 4/5.
func ParseNewCardRatio(num, den int) (NewCardRatio, error) {
	if den <= 0 {
		return NewCardRatio{}, ErrNewCardRatioDenominatorNotPositive
	}
	if num <= 0 || num >= den {
		return NewCardRatio{}, ErrNewCardRatioShareOutOfRange
	}
	g := int(new(big.Int).GCD(nil, nil, big.NewInt(int64(num)), big.NewInt(int64(den))).Int64())
	rnum, rden := num/g, den/g
	if rden > NewCardRatioDenMax {
		return NewCardRatio{}, ErrNewCardRatioDenominatorTooLarge
	}
	if rnum*NewCardRatioMaxNewShareDen > NewCardRatioMaxNewShareNum*rden {
		return NewCardRatio{}, ErrNewCardRatioNewShareTooHigh
	}
	return NewCardRatio{num: rnum, den: rden}, nil
}

func mustNewCardRatio(num, den int) NewCardRatio {
	r, err := ParseNewCardRatio(num, den)
	if err != nil {
		panic(err)
	}
	return r
}

// NewShare is the number of new cards per interleave cycle (interleave nRatio).
func (r NewCardRatio) NewShare() int { return r.num }

// ReviewShare is the number of review cards per interleave cycle (rRatio).
func (r NewCardRatio) ReviewShare() int { return r.den - r.num }

// Numerator / Denominator expose the reduced fraction for persistence and the
// wire form (numerator = new share, denominator = total).
func (r NewCardRatio) Numerator() int   { return r.num }
func (r NewCardRatio) Denominator() int { return r.den }

// IsZero reports whether r is the invalid zero value (never produced by
// ParseNewCardRatio). Read paths treat the zero value as "use the default".
func (r NewCardRatio) IsZero() bool { return r.den == 0 }
