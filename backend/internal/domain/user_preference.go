package domain

import "time"

// UserPreference is the per-user UI continuity bundle. A row exists only
// after the user has set at least one preference; absence means "all defaults".
type UserPreference struct {
	UserID                UserID
	LastViewedCardgroupID *string
	LearnDisplayMode      LearnDisplayMode
	NewCardRatio          NewCardRatio // zero value means "use DefaultNewCardRatio"
	UpdatedAt             time.Time
}
