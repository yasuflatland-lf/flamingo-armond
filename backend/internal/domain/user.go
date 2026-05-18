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

// UpdateProfile applies a parsed profile patch.
//
// DisplayName is always set (the VO contract guarantees 1..50 graphemes).
// Bio follows trinary semantics: when bio.IsSet() is false the field is left
// unchanged; otherwise bio.Value() (which may point at "") is written, allowing
// an explicit clear.
func (u *User) UpdateProfile(displayName DisplayName, bio Bio) {
	s := string(displayName)
	u.DisplayName = &s
	if bio.IsSet() {
		u.Bio = bio.Value()
	}
}
