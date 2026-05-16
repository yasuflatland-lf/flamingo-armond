package domain

import "time"

// User is the application-owned row keyed by auth.users.id.
type User struct {
	ID          string
	DisplayName *string
	Bio         *string
	AvatarURL   *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
