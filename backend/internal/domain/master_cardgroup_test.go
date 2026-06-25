package domain

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestNewMasterCardgroup(t *testing.T) {
	t.Parallel()

	t.Run("constructs a draft master cardgroup at version 1 with fields passed through", func(t *testing.T) {
		t.Parallel()

		name, err := ParseCardgroupName("Starter Deck")
		require.NoError(t, err)

		desc := "an intro deck"

		m, err := NewMasterCardgroup(name, DescriptionFromPtr(&desc), true, 5)
		require.NoError(t, err)
		require.NotNil(t, m)

		require.NotEmpty(t, m.ID, "constructor must generate an ID")
		require.Equal(t, name, m.Name)
		require.Equal(t, &desc, m.Description.Ptr())
		require.Equal(t, 1, m.Version, "a new master deck starts at version 1")
		require.Equal(t, MasterStatusDraft, m.Status, "a new master deck starts in draft")
		require.True(t, m.IsDefaultStarter)
		require.Equal(t, 5, m.SortOrder)
		require.False(t, m.CreatedAt.IsZero(), "constructor must stamp CreatedAt")
		require.Equal(t, m.CreatedAt, m.UpdatedAt, "CreatedAt and UpdatedAt must match at construction")
	})

	t.Run("nil optional fields stay nil and defaults hold", func(t *testing.T) {
		t.Parallel()

		name, err := ParseCardgroupName("Minimal")
		require.NoError(t, err)

		m, err := NewMasterCardgroup(name, Description{}, false, 0)
		require.NoError(t, err)
		require.Nil(t, m.Description.Ptr())
		require.Equal(t, 1, m.Version)
		require.Equal(t, MasterStatusDraft, m.Status)
		require.False(t, m.IsDefaultStarter)
		require.Equal(t, 0, m.SortOrder)
	})

	t.Run("name length invariant is surfaced upstream via CardgroupName", func(t *testing.T) {
		t.Parallel()

		// The constructor takes a parsed CardgroupName, so the 1..CardgroupNameMax
		// invariant is enforced by ParseCardgroupName before construction — mirroring
		// NewCardgroup. An over-long name never reaches NewMasterCardgroup.
		_, err := ParseCardgroupName(strings.Repeat("a", CardgroupNameMax+1))
		require.ErrorIs(t, err, ErrCardgroupNameTooLong)
	})
}

// TestNewMasterCardgroup_IDFailure pins the id-generation failure path through
// the newV7 test seam. Like TestNewCard_IDFailure it must NOT run in parallel:
// it swaps the package-level seam (see ids_test.go).
func TestNewMasterCardgroup_IDFailure(t *testing.T) {
	orig := newV7
	newV7 = func() (uuid.UUID, error) { return uuid.UUID{}, errors.New("crypto/rand unavailable") }
	t.Cleanup(func() { newV7 = orig })

	name, err := ParseCardgroupName("Starter")
	require.NoError(t, err)

	m, err := NewMasterCardgroup(name, Description{}, false, 0)
	require.Nil(t, m)
	require.Error(t, err)
	require.Contains(t, err.Error(), "master cardgroup: new id")
	require.Contains(t, err.Error(), "domain: new uuid v7")
}

func TestMasterCardgroupStatusIsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input MasterCardgroupStatus
		want  bool
	}{
		{"draft", MasterStatusDraft, true},
		{"published", MasterStatusPublished, true},
		{"empty", MasterCardgroupStatus(""), false},
		{"garbage", MasterCardgroupStatus("garbage"), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, tc.input.IsValid())
		})
	}
}

func TestMasterCardgroupPublish(t *testing.T) {
	t.Parallel()

	t.Run("draft transitions to published and bumps version by one", func(t *testing.T) {
		t.Parallel()
		m := &MasterCardgroup{Name: "n", Status: MasterStatusDraft, Version: 1}
		require.NoError(t, m.Publish())
		require.Equal(t, MasterStatusPublished, m.Status)
		require.Equal(t, 2, m.Version, "publish must increment version 1 -> 2")
	})

	t.Run("already published is idempotent and does not bump version", func(t *testing.T) {
		t.Parallel()
		m := &MasterCardgroup{Name: "n", Status: MasterStatusPublished, Version: 5}
		require.NoError(t, m.Publish())
		require.Equal(t, MasterStatusPublished, m.Status)
		require.Equal(t, 5, m.Version, "re-publishing an already-published group must not bump version")
	})
}

func TestMasterCardgroupUnpublish(t *testing.T) {
	t.Parallel()

	t.Run("published transitions to draft without changing version", func(t *testing.T) {
		t.Parallel()
		m := &MasterCardgroup{Name: "n", Status: MasterStatusPublished, Version: 3}
		require.NoError(t, m.Unpublish())
		require.Equal(t, MasterStatusDraft, m.Status)
		require.Equal(t, 3, m.Version, "unpublish must leave version unchanged")
	})

	t.Run("already draft stays draft with version unchanged", func(t *testing.T) {
		t.Parallel()
		m := &MasterCardgroup{Name: "n", Status: MasterStatusDraft, Version: 3}
		require.NoError(t, m.Unpublish())
		require.Equal(t, MasterStatusDraft, m.Status)
		require.Equal(t, 3, m.Version)
	})
}

func TestMasterCardgroupIsPublished(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status MasterCardgroupStatus
		want   bool
	}{
		{"published status returns true", MasterStatusPublished, true},
		{"draft status returns false", MasterStatusDraft, false},
		{"empty status returns false", MasterCardgroupStatus(""), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := &MasterCardgroup{
				Name:   CardgroupName(strings.Repeat("a", 1)),
				Status: tc.status,
			}
			require.Equal(t, tc.want, m.IsPublished())
		})
	}
}
