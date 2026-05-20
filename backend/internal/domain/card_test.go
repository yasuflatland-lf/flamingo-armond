package domain

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCardShape(t *testing.T) {
	t.Parallel()

	cardType := reflect.TypeFor[Card]()
	want := map[string]reflect.Type{
		"ID":          reflect.TypeFor[string](),
		"CardgroupID": reflect.TypeFor[string](),
		"Front":       reflect.TypeFor[CardText](),
		"Back":        reflect.TypeFor[CardText](),
		"CreatedAt":   reflect.TypeFor[time.Time](),
		"UpdatedAt":   reflect.TypeFor[time.Time](),
	}
	for name, typ := range want {
		field, ok := cardType.FieldByName(name)
		require.True(t, ok, "Card missing field %s", name)
		require.Equal(t, typ, field.Type, "Card.%s type mismatch", name)
		require.Empty(t, field.Tag, "Card.%s should not have struct tags", name)
	}
	_, ok := cardType.FieldByName("FSRS")
	require.False(t, ok, "Card must not embed per-user FSRS state")
}

func TestCardValidate(t *testing.T) {
	t.Parallel()

	const zwjEmoji = "👨‍👩‍👧‍👦"
	cases := []struct {
		name        string
		card        Card
		sentinelErr error
	}{
		{
			name:        "front required",
			card:        Card{CardgroupID: "cg", Front: "", Back: "back"},
			sentinelErr: ErrCardFrontRequired,
		},
		{
			name:        "front whitespace only treated as required",
			card:        Card{CardgroupID: "cg", Front: "   ", Back: "back"},
			sentinelErr: ErrCardFrontRequired,
		},
		{
			name:        "back required",
			card:        Card{CardgroupID: "cg", Front: "front", Back: "  "},
			sentinelErr: ErrCardBackRequired,
		},
		{
			name:        "front too long",
			card:        Card{CardgroupID: "cg", Front: CardText(strings.Repeat("a", 501)), Back: "back"},
			sentinelErr: ErrCardFrontTooLong,
		},
		{
			name:        "back too long with graphemes",
			card:        Card{CardgroupID: "cg", Front: "front", Back: CardText(strings.Repeat(zwjEmoji, 501))},
			sentinelErr: ErrCardBackTooLong,
		},
		{
			name: "valid at max grapheme length",
			card: Card{CardgroupID: "cg", Front: CardText(strings.Repeat(zwjEmoji, 500)), Back: CardText(strings.Repeat("b", 500))},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.card.Validate()
			if tc.sentinelErr == nil {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.True(t, errors.Is(err, tc.sentinelErr), "got %v", err)
		})
	}
}

func TestCardBelongsToCardgroup(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		card        Card
		cardgroupID string
		want        bool
	}{
		{
			name:        "matching cardgroup returns true",
			card:        Card{CardgroupID: "cg-1"},
			cardgroupID: "cg-1",
			want:        true,
		},
		{
			name:        "empty cardgroupID returns false",
			card:        Card{CardgroupID: "cg-1"},
			cardgroupID: "",
			want:        false,
		},
		{
			name:        "mismatched cardgroupID returns false",
			card:        Card{CardgroupID: "cg-1"},
			cardgroupID: "cg-2",
			want:        false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, tc.card.BelongsToCardgroup(tc.cardgroupID))
		})
	}
}

func TestNewFSRSStateForNewCard(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	got := NewFSRSStateForNewCard(now)

	require.Equal(t, now, got.Due)
	require.Equal(t, 2.5, got.Stability)
	require.Equal(t, 5.0, got.Difficulty)
	require.Zero(t, got.ElapsedDays)
	require.Zero(t, got.ScheduledDays)
	require.Zero(t, got.Reps)
	require.Zero(t, got.Lapses)
	require.Equal(t, FSRSStateNew, got.State)
	require.Equal(t, now, got.LastReview)
}

func TestRatingFromSwipeMode(t *testing.T) {
	t.Parallel()

	cases := []struct {
		mode    int
		want    Rating
		wantErr bool
	}{
		{mode: 1, want: RatingAgain},
		{mode: 2, want: RatingHard},
		{mode: 4, want: RatingEasy},
		{mode: 0, wantErr: true},
		{mode: 3, wantErr: true},
		{mode: 5, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("mode_%d", tc.mode), func(t *testing.T) {
			t.Parallel()
			got, err := RatingFromSwipeMode(tc.mode)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
