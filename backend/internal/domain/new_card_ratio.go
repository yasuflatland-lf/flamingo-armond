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
	den int // total; invariant den <= 100, gcd(num, den) == 1
}

// NewCardRatioDenMax caps the denominator so a stored ratio stays coarse enough
// for a human-facing setting (1% resolution is more than enough) and bounds the
// interleave loop counts.
const NewCardRatioDenMax = 100

// ParseNewCardRatio's rejection reasons, one sentinel per rule so callers can
// attribute the fault to a field with errors.Is instead of re-deriving the
// bounds. The bound-free reasons use plain errors.New so errors.Is matches by
// identity rather than by eris's message equality; the cap reason follows the
// bound-carrying sibling precedent (ErrRoleNameTooLong) and interpolates
// NewCardRatioDenMax so the exported bound and the message cannot drift apart.
var (
	ErrNewCardRatioDenominatorNotPositive = errors.New("domain: new card ratio: denominator must be positive")
	ErrNewCardRatioShareOutOfRange        = errors.New("domain: new card ratio: numerator must satisfy 0 < num < den")
	ErrNewCardRatioDenominatorTooLarge    = eris.Errorf("domain: new card ratio: reduced denominator exceeds max %d", NewCardRatioDenMax)
)

// DefaultNewCardRatio (4/5) is applied when a user has no stored preference.
// New share 4, review share 1 → interleave 4:1, matching the historical
// hard-coded 4:1 new:review interleave this VO replaces.
var DefaultNewCardRatio = mustNewCardRatio(4, 5)

// ParseNewCardRatio reduces num/den to lowest terms and validates the bounds.
// It returns a sentinel (not a silent fallback) on failure:
// ErrNewCardRatioDenominatorNotPositive for a non-positive denominator,
// ErrNewCardRatioShareOutOfRange for a share outside (0, den) — which covers a
// zero or negative numerator — and ErrNewCardRatioDenominatorTooLarge for a
// reduced denominator above NewCardRatioDenMax. The checks run in that order and
// return on the first failure, so exactly one sentinel is ever produced.
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
