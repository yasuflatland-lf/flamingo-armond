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

// Publish transitions the master cardgroup to the published state. It is
// idempotent: publishing an already-published group is a no-op and leaves the
// version untouched, so two concurrent publishes (serialized by the
// repository's row lock) increment the version exactly once. The version is
// bumped ONLY on the draft -> published transition; this asymmetry with
// Unpublish (which never touches the version) is the core publication rule and
// is encoded here, in the aggregate, rather than in repository SQL.
func (m *MasterCardgroup) Publish() error {
	if m.IsPublished() {
		return nil
	}
	m.Status = MasterStatusPublished
	m.Version++
	return nil
}

// Unpublish transitions the master cardgroup back to the draft state. The
// version is deliberately left unchanged — only Publish bumps it — so the
// version counter tracks publication events, not unpublications.
func (m *MasterCardgroup) Unpublish() error {
	m.Status = MasterStatusDraft
	return nil
}
