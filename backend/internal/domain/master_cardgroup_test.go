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
