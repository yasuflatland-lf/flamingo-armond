package domain

import "time"

// User is the application-owned row keyed by auth.users.id.
// It carries no DB tags so the repository layer owns persistence mapping.
type User struct {
	ID          string
	DisplayName *string
	Bio         *string
	AvatarURL   *string
	// LastViewedCardgroupID is the most recently viewed cardgroup on /learn,
	// or nil when never set. Backed by users.last_viewed_cardgroup_id with an
	// ON DELETE SET NULL FK so a deleted cardgroup nulls the column without
	// cascading to the user row.
	LastViewedCardgroupID *string
	// LastActive is the timestamp of the most recent authenticated request
	// recorded by the auth middleware. Nil when the user has never made an
	// authenticated request after the column was introduced.
	LastActive *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
