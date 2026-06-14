package domain

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

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
