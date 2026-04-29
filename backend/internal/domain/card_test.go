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

	cardType := reflect.TypeOf(Card{})
	want := map[string]reflect.Type{
		"ID":          reflect.TypeOf(""),
		"CardgroupID": reflect.TypeOf(""),
		"Front":       reflect.TypeOf(""),
		"Back":        reflect.TypeOf(""),
		"FSRS":        reflect.TypeOf(FSRSState{}),
		"CreatedAt":   reflect.TypeOf(time.Time{}),
		"UpdatedAt":   reflect.TypeOf(time.Time{}),
	}
	for name, typ := range want {
		field, ok := cardType.FieldByName(name)
		require.True(t, ok, "Card missing field %s", name)
		require.Equal(t, typ, field.Type, "Card.%s type mismatch", name)
		require.Empty(t, field.Tag, "Card.%s should not have struct tags", name)
	}
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
			name:        "cardgroup id required",
			card:        Card{Front: "front", Back: "back"},
			sentinelErr: ErrCardCardgroupIDRequired,
		},
		{
			name:        "front required",
			card:        Card{CardgroupID: "cg", Front: "", Back: "back"},
			sentinelErr: ErrCardFrontRequired,
		},
		{
			name:        "back required",
			card:        Card{CardgroupID: "cg", Front: "front", Back: "  "},
			sentinelErr: ErrCardBackRequired,
		},
		{
			name:        "front too long",
			card:        Card{CardgroupID: "cg", Front: strings.Repeat("a", 501), Back: "back"},
			sentinelErr: ErrCardFrontTooLong,
		},
		{
			name:        "back too long with graphemes",
			card:        Card{CardgroupID: "cg", Front: "front", Back: strings.Repeat(zwjEmoji, 501)},
			sentinelErr: ErrCardBackTooLong,
		},
		{
			name: "valid at max grapheme length",
			card: Card{CardgroupID: "cg", Front: strings.Repeat(zwjEmoji, 500), Back: strings.Repeat("b", 500)},
		},
	}

	for _, tc := range cases {
		tc := tc
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
		tc := tc
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
