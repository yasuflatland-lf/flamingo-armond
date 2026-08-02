package domain

import (
	"errors"
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
	{"ErrNewCardRatioNewShareTooHigh", ErrNewCardRatioNewShareTooHigh, false},
	{"ErrNewCardRatioDenominatorNotRepresentable", ErrNewCardRatioDenominatorNotRepresentable, false},
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

// TestDomainSentinels_MatchByIdentityNotMessage pins the declaration style itself.
// The cross-match matrix above only fires once two sentinels share a message, so it
// stays green if a single sentinel goes back to eris.New with its current unique text.
// An eris root answers errors.Is for ANY error carrying the same message, so comparing
// each sentinel against a freshly built twin of its own message discriminates the two
// declarations directly: errors.New says no, eris.New says yes.
func TestDomainSentinels_MatchByIdentityNotMessage(t *testing.T) {
	t.Parallel()

	for _, subject := range domainSentinels {
		if !subject.identityMatched {
			continue
		}
		t.Run(subject.name, func(t *testing.T) {
			t.Parallel()

			twin := errors.New(subject.err.Error())
			require.NotErrorIs(t, subject.err, twin,
				"%s matches a foreign error carrying the same message, so it is matched by message rather than by identity — declare it with errors.New, not eris.New",
				subject.name)
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
