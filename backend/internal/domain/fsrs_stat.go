package domain

// FSRSStat is a lightweight per-card projection of a user's FSRS state, consumed
// by the learning-stats domain services (AggregateMastery, TopStruggling). It
// carries no card front text — just CardID (row identity), CardgroupID (per-deck
// bucketing), Phase/Stability (the columns ClassifyMastery needs), and Lapses
// (the struggling-card ranking key).
//
// FSRSStat is not an aggregate; it is a view-level value shared between the
// repository (which builds it from a JOIN of user_card_fsrs and cards) and the
// learning-stats domain services (which consume it). It lives in the domain
// package because the mastery/struggle aggregation is domain logic and FSRSStat
// is its input — the same placement rule as DueCard. See
// docs/backend/ddd-patterns/view-level-value-in-domain-package.md.
//
// FSRSStat has no ParseFSRSStat constructor: the projection is established by
// the repository's SELECT/JOIN, not by a domain-side validator. Phase must be a
// valid FSRSPhase (the repository rejects out-of-range values before returning).
type FSRSStat struct {
	CardID      string
	CardgroupID string
	Phase       FSRSPhase
	Stability   float64
	Lapses      int
}
