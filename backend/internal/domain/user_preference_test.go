package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUserPreference_EffectiveNewCardRatio_ZeroFallsBackToDefault(t *testing.T) {
	t.Parallel()

	// The zero value is never produced by ParseNewCardRatio; a UserPreference
	// carrying it (a freshly built value, or a legacy row) resolves to the default.
	p := UserPreference{}
	require.True(t, p.NewCardRatio.IsZero())
	require.Equal(t, DefaultNewCardRatio, p.EffectiveNewCardRatio())
}

func TestUserPreference_EffectiveNewCardRatio_SetValueIsReturned(t *testing.T) {
	t.Parallel()

	set, err := ParseNewCardRatio(3, 4)
	require.NoError(t, err)
	p := UserPreference{NewCardRatio: set}
	require.Equal(t, set, p.EffectiveNewCardRatio())
}
