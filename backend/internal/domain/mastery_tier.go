package domain

// MatureStabilityDays is the FSRS stability (in days) at or above which a
// card is considered "mature" (durably retained). 21 days mirrors the Anki
// "mature card" cutoff (Anki thresholds on scheduled interval; we apply the
// same familiar numeric cutoff to FSRS stability).
const MatureStabilityDays = 21.0

// LearnedStabilityDays is the FSRS stability (in days) at or above which a
// card counts as learned/known: the model projects at least a week of
// retention. It is the single boundary both ClassifyMastery and the stats
// known-review gate test a card's stability against, so the mastery tiles and
// the retention/lapse population describe the same set of cards.
//
// That parity is conditional on the scheduler running in long-term mode.
// service.NewFSRSScheduler sets params.EnableShortTerm = false (see
// backend/internal/domain/service/fsrs_scheduler.go), so cards never sit in the
// Learning/Relearning phases. The stats known-review gate additionally requires
// PhaseBefore == Review, a conjunct ClassifyMastery does not test; parity holds
// for rows written under long-term mode. Rows last swiped under the legacy
// short-term scheduler (pre 4a92c9f4) can sit in Learning/Relearning with
// stability at or above this constant: the mastery tiles count them as learned
// while their next swipe is excluded from the retention/lapse population. The
// divergence self-heals on that swipe, which moves the row into Review. (A
// first-ever swipe does fail it, carrying PhaseBefore == FSRSPhaseNew, but
// its stability is below the boundary, so both populations exclude it and the
// parity holds.) Flipping
// EnableShortTerm back to true would admit a card with a non-Review phase and
// stability at or above this constant: the mastery tiles
// would still count it as learned while the retention/lapse population dropped
// it, and the two would diverge with no compile error and no failing test at
// this constant's own site. Revisit this boundary if the scheduler mode changes.
const LearnedStabilityDays = 7.0

// MasteryTier is the disjoint learning tier a card sits in.
type MasteryTier int

const (
	TierInProgress MasteryTier = iota // Stability below the learned threshold (still being acquired, or recently lapsed)
	TierLearned                       // Stability at or above the learned threshold and below the mature threshold
	TierMature                        // Stability at or above the mature threshold
)

// ClassifyMastery buckets an FSRS state into exactly one tier by memory
// stability. Thresholds are passed (not read from the consts) so tests can pin
// the boundaries.
func ClassifyMastery(state FSRSState, learnedStabilityDays, matureStabilityDays float64) MasteryTier {
	if state.Stability < learnedStabilityDays {
		return TierInProgress
	}
	if state.Stability >= matureStabilityDays {
		return TierMature
	}
	return TierLearned
}
