package domain

// MatureStabilityDays is the FSRS stability (in days) at or above which a
// card is considered "mature" (durably retained). 21 days mirrors the Anki
// "mature card" cutoff (Anki thresholds on scheduled interval; we apply the
// same familiar numeric cutoff to FSRS stability).
// Under the shipped configuration the two readings coincide: go-fsrs derives an
// interval as s/factor * (RequestRetention^(1/decay) - 1) with
// factor = 0.9^(1/decay) - 1, so at the default RequestRetention of exactly 0.9
// the interval IS the stability, and ScheduledDays is round(stability) clamped to
// [1, MaximumInterval]. That identity is what makes 21 mean the same thing on both
// scales, and it breaks for any other retention — revisit this constant if
// RequestRetention ever becomes configurable.
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
// for rows written under long-term mode. A first-ever swipe does fail it,
// carrying PhaseBefore == FSRSPhaseNew, but its stability is below the boundary,
// so both populations exclude it and the parity holds.
//
// The flag is not reachable only by a deliberate edit: fsrs.NewFSRS replaces the
// caller's whole Parameters value with its defaults when validation fails, and
// those defaults enable short-term mode. service.NewFSRSScheduler asserts the
// flag after construction precisely so that path cannot reach this constant
// silently. Either way in — a deliberate flip or a discarded parameter — admits a
// card with a non-Review phase and stability at or above this constant: the
// mastery tiles would still count it as learned while the retention/lapse
// population dropped it, with no compile error and no failing test at this
// constant's own site. Revisit this boundary if the scheduler mode changes.
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
