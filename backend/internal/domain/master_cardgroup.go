package domain

import "time"

// MasterCardgroupStatus is the publication lifecycle state of a master cardgroup.
// The zero value (MasterCardgroupStatus("")) is invalid; use MasterStatusDraft or
// MasterStatusPublished.
type MasterCardgroupStatus string

const (
	MasterStatusDraft     MasterCardgroupStatus = "draft"
	MasterStatusPublished MasterCardgroupStatus = "published"
)

// IsValid reports whether s is a recognised MasterCardgroupStatus constant.
func (s MasterCardgroupStatus) IsValid() bool {
	return s == MasterStatusDraft || s == MasterStatusPublished
}

// MasterCardgroup is the admin-managed template for a deck of cards. It is
// distinct from the user-owned Cardgroup aggregate: a MasterCardgroup lives in
// the master catalog and is never directly owned by an end user.
//
// Name is a CardgroupName so the same grapheme-cluster length invariant
// (1..CardgroupNameMax) applies without duplicating validation logic.
type MasterCardgroup struct {
	ID               string
	Name             CardgroupName
	Description      *string
	Language         *string
	Level            *string
	Category         *string
	CoverImageURL    *string
	Source           *string
	Version          int
	Status           MasterCardgroupStatus
	IsDefaultStarter bool
	SortOrder        int
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// IsPublished reports whether the master cardgroup is in the published state
// and therefore visible to end users.
func (m *MasterCardgroup) IsPublished() bool {
	return m.Status == MasterStatusPublished
}
