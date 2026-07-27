package domain

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestNewMasterCard(t *testing.T) {
	t.Parallel()

	const zwjEmoji = "👨‍👩‍👧‍👦"
	now := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)

	t.Run("valid input constructs a fully-formed master card", func(t *testing.T) {
		t.Parallel()

		c, err := NewMasterCard("mcg", "  front  ", "back", 3, now)
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

			c, err := NewMasterCard("mcg", tc.front, tc.back, 0, now)
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

	now := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	c, err := NewMasterCard("mcg", "front", "back", 0, now)
	require.Nil(t, c)
	require.Error(t, err)
	require.Contains(t, err.Error(), "master card: new id")
	require.Contains(t, err.Error(), "domain: new uuid v7")
}

func TestMasterCardBelongsToMasterCardgroup(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name              string
		card              MasterCard
		masterCardgroupID string
		want              bool
	}{
		{
			name:              "matching master cardgroup returns true",
			card:              MasterCard{MasterCardgroupID: "mcg-1"},
			masterCardgroupID: "mcg-1",
			want:              true,
		},
		{
			name:              "empty masterCardgroupID returns false",
			card:              MasterCard{MasterCardgroupID: "mcg-1"},
			masterCardgroupID: "",
			want:              false,
		},
		{
			name:              "mismatched masterCardgroupID returns false",
			card:              MasterCard{MasterCardgroupID: "mcg-1"},
			masterCardgroupID: "mcg-2",
			want:              false,
		},
		{
			name:              "both empty returns false",
			card:              MasterCard{MasterCardgroupID: ""},
			masterCardgroupID: "",
			want:              false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, tc.card.BelongsToMasterCardgroup(tc.masterCardgroupID))
		})
	}
}

func TestMasterCardUpdateFront(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		initial   MasterCard
		newFront  CardText
		wantErr   error
		wantFront CardText
		wantBack  CardText
	}{
		{
			name:      "empty CardText rejected with ErrCardFrontRequired",
			initial:   MasterCard{ID: "c", MasterCardgroupID: "mcg", Front: "front", Back: "back"},
			newFront:  "",
			wantErr:   ErrCardFrontRequired,
			wantFront: "front", // unchanged on error
			wantBack:  "back",
		},
		{
			name:      "valid CardText updates Front and returns nil",
			initial:   MasterCard{ID: "c", MasterCardgroupID: "mcg", Front: "front", Back: "back"},
			newFront:  "new front",
			wantErr:   nil,
			wantFront: "new front",
			wantBack:  "back",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			card := tc.initial
			err := card.UpdateFront(tc.newFront)

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.wantFront, card.Front)
			// Back must remain untouched regardless of outcome.
			require.Equal(t, tc.wantBack, card.Back)
		})
	}
}

func TestMasterCardUpdateBack(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		initial   MasterCard
		newBack   CardText
		wantErr   error
		wantFront CardText
		wantBack  CardText
	}{
		{
			name:      "empty CardText rejected with ErrCardBackRequired",
			initial:   MasterCard{ID: "c", MasterCardgroupID: "mcg", Front: "front", Back: "back"},
			newBack:   "",
			wantErr:   ErrCardBackRequired,
			wantFront: "front",
			wantBack:  "back", // unchanged on error
		},
		{
			name:      "valid CardText updates Back and returns nil",
			initial:   MasterCard{ID: "c", MasterCardgroupID: "mcg", Front: "front", Back: "back"},
			newBack:   "new back",
			wantErr:   nil,
			wantFront: "front",
			wantBack:  "new back",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			card := tc.initial
			err := card.UpdateBack(tc.newBack)

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
			// Front must remain untouched regardless of outcome.
			require.Equal(t, tc.wantFront, card.Front)
			require.Equal(t, tc.wantBack, card.Back)
		})
	}
}

func TestNewMasterCardFromValidated(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)

	t.Run("valid VOs construct a fully-formed master card without re-validation", func(t *testing.T) {
		t.Parallel()

		c, err := NewMasterCardFromValidated("mcg-1", CardText("front"), CardText("back"), 7, now)
		require.NoError(t, err)
		require.NotEmpty(t, c.ID, "constructor must generate an ID")
		require.Equal(t, "mcg-1", c.MasterCardgroupID)
		require.Equal(t, CardText("front"), c.Front)
		require.Equal(t, CardText("back"), c.Back)
		require.Equal(t, 7, c.Position)
		require.Equal(t, now, c.CreatedAt)
		require.Equal(t, now, c.UpdatedAt)
	})

	t.Run("VOs are stored verbatim, not re-parsed", func(t *testing.T) {
		t.Parallel()

		// A CardText VO with surrounding whitespace can only exist if a caller
		// bypasses ParseCardText; NewMasterCardFromValidated must not trim it,
		// proving it skips the second grapheme scan NewMasterCard performs.
		c, err := NewMasterCardFromValidated("mcg", CardText("  raw  "), CardText("back"), 0, now)
		require.NoError(t, err)
		require.Equal(t, CardText("  raw  "), c.Front, "front must be stored verbatim, not re-trimmed")
	})

	cases := []struct {
		name        string
		front       CardText
		back        CardText
		sentinelErr error
	}{
		{"zero front VO rejected", CardText(""), CardText("back"), ErrCardFrontRequired},
		{"zero back VO rejected", CardText("front"), CardText(""), ErrCardBackRequired},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c, err := NewMasterCardFromValidated("mcg", tc.front, tc.back, 0, now)
			require.Nil(t, c, "no aggregate may be constructed from a zero-value CardText")
			require.ErrorIs(t, err, tc.sentinelErr, "got %v", err)
		})
	}
}

// TestNewMasterCardFromValidated_IDFailure pins the id-generation failure path
// through the newV7 test seam; the non-zero VO checks run first, so valid VOs
// reach NewID.
func TestNewMasterCardFromValidated_IDFailure(t *testing.T) {
	orig := newV7
	newV7 = func() (uuid.UUID, error) { return uuid.UUID{}, errors.New("crypto/rand unavailable") }
	t.Cleanup(func() { newV7 = orig })

	now := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	c, err := NewMasterCardFromValidated("mcg", CardText("front"), CardText("back"), 0, now)
	require.Nil(t, c)
	require.Error(t, err)
	require.Contains(t, err.Error(), "master card: new id")
	require.Contains(t, err.Error(), "domain: new uuid v7")
}
