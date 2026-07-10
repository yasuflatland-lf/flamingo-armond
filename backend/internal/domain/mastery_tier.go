package domain

// MatureStabilityDays is the FSRS stability (in days) at or above which a
// Review-phase card is considered "mature" (durably retained). 21 days mirrors
// the Anki "mature card" convention. Single tunable; the three-tier definition
// lives only in ClassifyMastery.
const MatureStabilityDays = 21.0

// MasteryTier is the disjoint learning tier a card sits in.
type MasteryTier int

const (
	TierInProgress MasteryTier = iota // New / Learning / Relearning
	TierLearned                       // Review, stability < threshold
	TierMature                        // Review, stability >= threshold
)

// ClassifyMastery buckets an FSRS state into exactly one tier. matureStabilityDays
// is passed (not read from the const) so tests can pin the boundary.
func ClassifyMastery(state FSRSState, matureStabilityDays float64) MasteryTier {
	if state.Phase != FSRSPhaseReview {
		return TierInProgress
	}
	if state.Stability >= matureStabilityDays {
		return TierMature
	}
	return TierLearned
}
