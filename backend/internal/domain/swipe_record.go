package domain

import (
	"time"

	"github.com/rotisserie/eris"
)

// SwipeRecord is an immutable domain event. Append-only; never updated.
type SwipeRecord struct {
	ID          string
	UserID      UserID
	CardID      string
	CardgroupID CardgroupID
	Rating      Rating
	ReviewedAt  time.Time
	StateAfter  FSRSState
	// PhaseBefore is the FSRS phase the card was in when the swipe was made;
	// nil for rows recorded before the phase_before column existed.
	PhaseBefore *FSRSPhase
	// ScheduledDaysBefore is the interval (days) scheduled at the previous
	// review; nil for legacy rows recorded before the column existed.
	ScheduledDaysBefore *int
	// StabilityBefore is the FSRS stability (days) the card held before this
	// swipe; nil for rows recorded before the column existed.
	StabilityBefore *float64
}

// NewSwipeRecord creates a swipe event with a fresh UUID v7. stateBefore is the
// scheduling state captured immediately before the rating was applied; the
// pre-swipe snapshot (PhaseBefore, ScheduledDaysBefore, StabilityBefore) is
// populated from it so the metrics layer can distinguish, for example, a
// Learning->Easy graduation from a genuine review of an already-learned card.
// stateAfter is the state the swipe advanced the card to.
func NewSwipeRecord(userID UserID, cardID string, cardgroupID CardgroupID, rating Rating, reviewedAt time.Time, stateBefore, stateAfter FSRSState) (*SwipeRecord, error) {
	if cardgroupID == "" {
		return nil, eris.New("swipe record: cardgroupID is required")
	}
	id, err := NewID()
	if err != nil {
		return nil, eris.Wrap(err, "swipe record: new id")
	}
	// Take the address of independent locals, never &stateBefore.Field: the
	// snapshot pointers must not alias the caller's FSRSState.
	phaseBefore := stateBefore.Phase
	scheduledDaysBefore := stateBefore.ScheduledDays
	stabilityBefore := stateBefore.Stability
	return &SwipeRecord{
		ID:                  id,
		UserID:              userID,
		CardID:              cardID,
		CardgroupID:         cardgroupID,
		Rating:              rating,
		ReviewedAt:          reviewedAt,
		StateAfter:          stateAfter,
		PhaseBefore:         &phaseBefore,
		ScheduledDaysBefore: &scheduledDaysBefore,
		StabilityBefore:     &stabilityBefore,
	}, nil
}
