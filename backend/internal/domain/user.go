package domain

import "time"

// User is the application-owned row keyed by auth.users.id.
// It carries no DB tags so the repository layer owns persistence mapping.
type User struct {
	ID          string
	DisplayName *string
	Bio         *string
	AvatarURL   *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
