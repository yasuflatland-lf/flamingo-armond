package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewSwipeRecord(t *testing.T) {
	t.Parallel()

	reviewedAt := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	stateBefore := NewFSRSStateForNewCard(reviewedAt)
	stateBefore.Phase = FSRSPhaseReview
	stateBefore.ScheduledDays = 7
	stateBefore.Stability = 6.6
	stateBefore.Due = reviewedAt.Add(7 * 24 * time.Hour)
	stateAfter := NewFSRSStateForNewCard(reviewedAt)
	stateAfter.Phase = FSRSPhaseRelearning
	stateAfter.ScheduledDays = 1

	first, err := NewSwipeRecord("user-1", "card-1", CardgroupID("cg-1"), RatingEasy, reviewedAt, stateBefore, stateAfter)
	require.NoError(t, err)
	second, err := NewSwipeRecord("user-1", "card-1", CardgroupID("cg-1"), RatingEasy, reviewedAt, stateBefore, stateAfter)
	require.NoError(t, err)

	require.NotEmpty(t, first.ID)
	require.NotEqual(t, first.ID, second.ID)
	require.Equal(t, UserID("user-1"), first.UserID)
	require.Equal(t, "card-1", first.CardID)
	require.Equal(t, CardgroupID("cg-1"), first.CardgroupID)
	require.Equal(t, RatingEasy, first.Rating)
	require.Equal(t, reviewedAt, first.ReviewedAt)
	require.Equal(t, stateAfter, first.StateAfter)

	// The pre-swipe snapshot is taken from stateBefore, never from stateAfter.
	require.Equal(t, FSRSPhaseReview, first.PhaseBefore)
	require.InDelta(t, 6.6, first.StabilityBefore, 0.000000001)
	require.Equal(t, stateBefore.Due, first.DueBefore)
}

func TestNewSwipeRecord_EmptyCardgroupID(t *testing.T) {
	t.Parallel()

	reviewedAt := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	state := NewFSRSStateForNewCard(reviewedAt)

	_, err := NewSwipeRecord("user-1", "card-1", CardgroupID(""), RatingEasy, reviewedAt, state, state)
	require.EqualError(t, err, "swipe record: cardgroupID is required")
}
