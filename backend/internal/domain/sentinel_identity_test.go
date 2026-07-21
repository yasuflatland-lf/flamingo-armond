package domain

import (
	"testing"

	"github.com/rotisserie/eris"
	"github.com/stretchr/testify/require"
)

// domainSentinel names one package-level validation sentinel. identityMatched
// marks the sentinels declared with plain errors.New, which errors.Is resolves by
// pointer identity; the rest interpolate an exported bound and stay on
// eris.Errorf, which errors.Is resolves by message equality.
type domainSentinel struct {
	name            string
	err             error
	identityMatched bool
}

// domainSentinels lists every package-level validation sentinel in this package.
// The cross-match matrix runs over the whole list rather than the identity-matched
// subset alone, because eris compares messages: a duplicated message on any pair
// makes errors.Is answer true for the wrong sentinel, and the per-aggregate tables
// are positive-only so the duplicate would make a new assertion pass instead of
// making an existing one fail.
var domainSentinels = []domainSentinel{
	{"ErrCardFrontRequired", ErrCardFrontRequired, true},
	{"ErrCardFrontTooLong", ErrCardFrontTooLong, false},
	{"ErrCardBackRequired", ErrCardBackRequired, true},
	{"ErrCardBackTooLong", ErrCardBackTooLong, false},
	{"ErrCardgroupNameRequired", ErrCardgroupNameRequired, true},
	{"ErrCardgroupNameTooLong", ErrCardgroupNameTooLong, false},
	{"ErrDisplayNameRequired", ErrDisplayNameRequired, true},
	{"ErrDisplayNameTooLong", ErrDisplayNameTooLong, false},
	{"ErrDisplayNameReserved", ErrDisplayNameReserved, true},
	{"ErrRoleNameRequired", ErrRoleNameRequired, true},
	{"ErrRoleNameTooLong", ErrRoleNameTooLong, false},
	{"ErrRoleNameInvalid", ErrRoleNameInvalid, true},
	{"ErrBioTooLong", ErrBioTooLong, false},
	{"ErrDescriptionTooLong", ErrDescriptionTooLong, false},
	{"ErrNewCardRatioDenominatorNotPositive", ErrNewCardRatioDenominatorNotPositive, true},
	{"ErrNewCardRatioShareOutOfRange", ErrNewCardRatioShareOutOfRange, true},
	{"ErrNewCardRatioDenominatorTooLarge", ErrNewCardRatioDenominatorTooLarge, false},
}

func TestDomainSentinels_DoNotCrossMatch(t *testing.T) {
	t.Parallel()

	for _, subject := range domainSentinels {
		t.Run(subject.name, func(t *testing.T) {
			t.Parallel()

			require.ErrorIs(t, subject.err, subject.err, "a sentinel must match itself")
			for _, other := range domainSentinels {
				if other.name == subject.name {
					continue
				}
				require.NotErrorIs(t, subject.err, other.err,
					"%s must not match %s — a shared message string would make errors.Is attribute the fault to the wrong field",
					subject.name, other.name)
			}
		})
	}
}

func TestDomainSentinels_SurviveErisWrap(t *testing.T) {
	t.Parallel()

	for _, subject := range domainSentinels {
		if !subject.identityMatched {
			continue
		}
		t.Run(subject.name, func(t *testing.T) {
			t.Parallel()

			wrapped := eris.Wrap(subject.err, "usecase: card: create")
			require.ErrorIs(t, wrapped, subject.err,
				"an eris.Wrap of %s must still satisfy errors.Is", subject.name)
			for _, other := range domainSentinels {
				if other.name == subject.name {
					continue
				}
				require.NotErrorIs(t, wrapped, other.err,
					"an eris.Wrap of %s must not match %s", subject.name, other.name)
			}
		})
	}
}
