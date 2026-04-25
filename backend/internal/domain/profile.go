package domain

import "time"

// Profile is the application-level representation of a user profile row.
// It carries no DB tags so the repository layer owns the mapping.
type Profile struct {
	ID          string
	DisplayName *string
	Bio         *string
	AvatarURL   *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
