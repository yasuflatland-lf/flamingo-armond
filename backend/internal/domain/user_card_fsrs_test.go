package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type stubScheduler struct {
	gotState  FSRSState
	gotRating Rating
	gotNow    time.Time
	out       FSRSState
	called    int
}

func (s *stubScheduler) Apply(state FSRSState, rating Rating, now time.Time) FSRSState {
	s.gotState = state
	s.gotRating = rating
	s.gotNow = now
	s.called++
	return s.out
}

func TestUserCardFSRSOrNew(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)
	userID := UserID("u-1")
	cardID := "card-1"

	t.Run("nil record returns the default new-card state", func(t *testing.T) {
		t.Parallel()

		got := UserCardFSRSOrNew(nil, userID, cardID, createdAt)
		want := NewUserCardFSRSForNewCard(userID, cardID, createdAt)

		require.Equal(t, want, got)
		require.Equal(t, userID, got.UserID)
		require.Equal(t, cardID, got.CardID)
		require.Equal(t, FSRSPhaseNew, got.State.Phase)
		require.Equal(t, createdAt, got.State.LastReview)
	})

	t.Run("non-nil record returns the same pointer", func(t *testing.T) {
		t.Parallel()

		existing := NewUserCardFSRSForNewCard(UserID("u-9"), "card-9", createdAt.Add(-time.Hour))
		got := UserCardFSRSOrNew(existing, userID, cardID, createdAt)

		require.Same(t, existing, got)
	})
}

func TestUserCardFSRS_ApplyRating_HappyPath(t *testing.T) {
	t.Parallel()

	t0 := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Hour)

	outState := FSRSState{
		Reps:       1,
		Stability:  3.0,
		Difficulty: 4.5,
		Phase:      FSRSPhaseLearning,
		LastReview: t1,
	}
	stub := &stubScheduler{out: outState}

	u := NewUserCardFSRSForNewCard("u1", "c1", t0)
	err := u.ApplyRating(stub, RatingGood, t1)

	require.NoError(t, err)
	require.Equal(t, outState, u.State)
	require.Equal(t, t1, u.UpdatedAt)
	require.Equal(t, RatingGood, stub.gotRating)
	require.Equal(t, t1, stub.gotNow)
}

func TestUserCardFSRS_ApplyRating_InvalidRating(t *testing.T) {
	t.Parallel()

	t0 := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Hour)

	cases := []struct {
		name   string
		rating Rating
	}{
		{name: "zero", rating: Rating(0)},
		{name: "too_high", rating: Rating(5)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			stub := &stubScheduler{}
			u := NewUserCardFSRSForNewCard("u1", "c1", t0)
			snapshotState := u.State
			snapshotUpdatedAt := u.UpdatedAt

			err := u.ApplyRating(stub, tc.rating, t1)

			require.Error(t, err)
			require.Equal(t, snapshotState, u.State)
			require.Equal(t, snapshotUpdatedAt, u.UpdatedAt)
			require.Equal(t, 0, stub.called)
		})
	}
}
