package domain

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestNewMasterCard(t *testing.T) {
	t.Parallel()

	const zwjEmoji = "👨‍👩‍👧‍👦"

	t.Run("valid input constructs a fully-formed master card", func(t *testing.T) {
		t.Parallel()

		c, err := NewMasterCard("mcg", "  front  ", "back", 3)
		require.NoError(t, err)
		require.NotEmpty(t, c.ID, "constructor must generate an ID")
		require.Equal(t, "mcg", c.MasterCardgroupID)
		require.Equal(t, CardText("front"), c.Front, "front must be trimmed via ParseCardText")
		require.Equal(t, CardText("back"), c.Back)
		require.Equal(t, 3, c.Position)
		require.False(t, c.CreatedAt.IsZero(), "constructor must stamp CreatedAt")
		require.Equal(t, c.CreatedAt, c.UpdatedAt, "CreatedAt and UpdatedAt must match at construction")
	})

	cases := []struct {
		name        string
		front       string
		back        string
		sentinelErr error
	}{
		{"empty front returns ErrCardFrontRequired", "", "back", ErrCardFrontRequired},
		{"whitespace-only front returns ErrCardFrontRequired", "   ", "back", ErrCardFrontRequired},
		{"front over CardTextMax returns ErrCardFrontTooLong", strings.Repeat("a", CardTextMax+1), "back", ErrCardFrontTooLong},
		{"front over CardTextMax with graphemes returns ErrCardFrontTooLong", strings.Repeat(zwjEmoji, CardTextMax+1), "back", ErrCardFrontTooLong},
		{"empty back returns ErrCardBackRequired", "front", "", ErrCardBackRequired},
		{"back over CardTextMax returns ErrCardBackTooLong", "front", strings.Repeat("b", CardTextMax+1), ErrCardBackTooLong},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c, err := NewMasterCard("mcg", tc.front, tc.back, 0)
			require.Nil(t, c, "no aggregate may be constructed from invalid input")
			require.ErrorIs(t, err, tc.sentinelErr, "got %v", err)
		})
	}
}

// TestNewMasterCard_IDFailure pins the id-generation failure path through the
// newV7 test seam; ParseCardText runs first, so valid text reaches NewID.
func TestNewMasterCard_IDFailure(t *testing.T) {
	orig := newV7
	newV7 = func() (uuid.UUID, error) { return uuid.UUID{}, errors.New("crypto/rand unavailable") }
	t.Cleanup(func() { newV7 = orig })

	c, err := NewMasterCard("mcg", "front", "back", 0)
	require.Nil(t, c)
	require.Error(t, err)
	require.Contains(t, err.Error(), "master card: new id")
	require.Contains(t, err.Error(), "domain: new uuid v7")
}
