package domain

// MatureStabilityDays is the FSRS stability (in days) at or above which a
// card is considered "mature" (durably retained). 21 days mirrors the Anki
// "mature card" cutoff (Anki thresholds on scheduled interval; we apply the
// same familiar numeric cutoff to FSRS stability).
const MatureStabilityDays = 21.0

// LearnedStabilityDays is the FSRS stability (in days) at or above which a
// card counts as learned/known: the model projects at least a week of
// retention. Under the default RequestRetention=0.9 the scheduled interval
// equals round(stability), so a swipe's ScheduledDaysBefore snapshot reads
// the same boundary at swipe time.
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
