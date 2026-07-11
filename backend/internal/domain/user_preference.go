package domain

import "time"

// UserPreference is the per-user UI continuity bundle. A row exists only
// after the user has set at least one preference; absence means "all defaults".
type UserPreference struct {
	UserID                UserID
	LastViewedCardgroupID *string
	LearnDisplayMode      LearnDisplayMode
	NewCardRatio          NewCardRatio // zero value means "use DefaultNewCardRatio"; read via EffectiveNewCardRatio
	UpdatedAt             time.Time
}

// EffectiveNewCardRatio resolves the stored NewCardRatio to the ratio a read
// path should apply, substituting DefaultNewCardRatio for the invalid zero
// value. This method is the single home of the "zero means default" rule; read
// sites (the learn usecase and the newCardRatio resolver) call it instead of
// re-checking NewCardRatio.IsZero() themselves. The repository mapper's own
// pre-defaulting in toDomainUserPreference is a DB-read normalization of
// out-of-bounds or legacy-zero column values, not a second owner of this rule.
func (p UserPreference) EffectiveNewCardRatio() NewCardRatio {
	if p.NewCardRatio.IsZero() {
		return DefaultNewCardRatio
	}
	return p.NewCardRatio
}
