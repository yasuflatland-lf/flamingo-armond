package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewSwipeRecord(t *testing.T) {
	t.Parallel()

	reviewedAt := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	state := NewFSRSStateForNewCard(reviewedAt)

	first, err := NewSwipeRecord("user-1", "card-1", CardgroupID("cg-1"), RatingEasy, reviewedAt, state)
	require.NoError(t, err)
	second, err := NewSwipeRecord("user-1", "card-1", CardgroupID("cg-1"), RatingEasy, reviewedAt, state)
	require.NoError(t, err)

	require.NotEmpty(t, first.ID)
	require.NotEqual(t, first.ID, second.ID)
	require.Equal(t, UserID("user-1"), first.UserID)
	require.Equal(t, "card-1", first.CardID)
	require.Equal(t, CardgroupID("cg-1"), first.CardgroupID)
	require.Equal(t, RatingEasy, first.Rating)
	require.Equal(t, reviewedAt, first.ReviewedAt)
	require.Equal(t, state, first.StateAfter)
}

func TestNewSwipeRecord_EmptyCardgroupID(t *testing.T) {
	t.Parallel()

	reviewedAt := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)

	_, err := NewSwipeRecord("user-1", "card-1", CardgroupID(""), RatingEasy, reviewedAt, NewFSRSStateForNewCard(reviewedAt))
	require.EqualError(t, err, "swipe record: cardgroupID is required")
}
