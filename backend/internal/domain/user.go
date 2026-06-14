package domain

import "time"

// User is the application-owned row keyed by auth.users.id.
//
// DisplayName is *DisplayName so a NULL column round-trips as a nil pointer
// (no display name set). Bio is the trinary VO Bio (by value). The zero value
// Bio{} represents a NULL bio column (IsSet()=false); a set Bio carries either
// an explicit empty-string value or non-empty text. The trinary's "no change"
// meaning applies in the UpdateProfileInput patch context, not here.
type User struct {
	ID          UserID
	DisplayName *DisplayName
	Bio         Bio
	AvatarURL   *string
	Version     int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
