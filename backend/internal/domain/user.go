package domain

import "time"

// User is the application-owned row keyed by auth.users.id.
//
// DisplayName is *DisplayName so a NULL column round-trips as a nil pointer
// (no display name set). Bio is the trinary VO Bio (by value): the zero value
// Bio{} encodes "no change" / NULL via IsSet()=false, while a set Bio encodes
// either an explicit clear (empty string) or a non-empty value.
type User struct {
	ID          string
	DisplayName *DisplayName
	Bio         Bio
	AvatarURL   *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
