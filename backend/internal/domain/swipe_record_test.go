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
	require.NotNil(t, first.PhaseBefore)
	require.Equal(t, FSRSPhaseReview, *first.PhaseBefore)
	require.NotNil(t, first.ScheduledDaysBefore)
	require.Equal(t, 7, *first.ScheduledDaysBefore)
	require.NotNil(t, first.StabilityBefore)
	require.InDelta(t, 6.6, *first.StabilityBefore, 0.000000001)
}

// TestNewSwipeRecord_SnapshotPointersAreIndependentCopies proves the snapshot
// pointers are fresh allocations that do not alias the caller's stateBefore, so
// a later mutation of the caller's struct cannot corrupt the recorded event.
// Pointer-identity assertion per
// docs/backend/library-gotchas/defensive-copy-test-pointer-identity.md.
func TestNewSwipeRecord_SnapshotPointersAreIndependentCopies(t *testing.T) {
	t.Parallel()

	reviewedAt := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	stateBefore := NewFSRSStateForNewCard(reviewedAt)
	stateBefore.Phase = FSRSPhaseReview
	stateBefore.ScheduledDays = 7
	stateBefore.Stability = 6.6
	stateAfter := NewFSRSStateForNewCard(reviewedAt)

	rec, err := NewSwipeRecord("user-1", "card-1", CardgroupID("cg-1"), RatingGood, reviewedAt, stateBefore, stateAfter)
	require.NoError(t, err)

	require.NotSame(t, &stateBefore.Phase, rec.PhaseBefore, "PhaseBefore must not alias the caller's stateBefore")
	require.NotSame(t, &stateBefore.ScheduledDays, rec.ScheduledDaysBefore, "ScheduledDaysBefore must not alias the caller's stateBefore")
	require.NotSame(t, &stateBefore.Stability, rec.StabilityBefore, "StabilityBefore must not alias the caller's stateBefore")

	// Mutating the caller's copy after construction must not change the record.
	stateBefore.Phase = FSRSPhaseNew
	stateBefore.ScheduledDays = 999
	stateBefore.Stability = 999
	require.Equal(t, FSRSPhaseReview, *rec.PhaseBefore, "PhaseBefore is an independent copy")
	require.Equal(t, 7, *rec.ScheduledDaysBefore, "ScheduledDaysBefore is an independent copy")
	require.InDelta(t, 6.6, *rec.StabilityBefore, 0.000000001, "StabilityBefore is an independent copy")
}

func TestNewSwipeRecord_EmptyCardgroupID(t *testing.T) {
	t.Parallel()

	reviewedAt := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	state := NewFSRSStateForNewCard(reviewedAt)

	_, err := NewSwipeRecord("user-1", "card-1", CardgroupID(""), RatingEasy, reviewedAt, state, state)
	require.EqualError(t, err, "swipe record: cardgroupID is required")
}
