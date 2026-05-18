package domain

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseRoleName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		input       string
		wantValue   RoleName
		sentinelErr error
	}{
		{
			name:      "admin lowercase",
			input:     "admin",
			wantValue: "admin",
		},
		{
			name:      "general lowercase",
			input:     "general",
			wantValue: "general",
		},
		{
			name:      "uppercase normalized",
			input:     "ADMIN",
			wantValue: "admin",
		},
		{
			name:      "whitespace trimmed",
			input:     "  admin  ",
			wantValue: "admin",
		},
		{
			name:        "empty string",
			input:       "",
			sentinelErr: ErrRoleNameRequired,
		},
		{
			name:        "whitespace only",
			input:       "   ",
			sentinelErr: ErrRoleNameRequired,
		},
		{
			name:        "too long",
			input:       strings.Repeat("a", 51),
			sentinelErr: ErrRoleNameTooLong,
		},
		{
			name:        "invalid character exclamation",
			input:       "admin!",
			sentinelErr: ErrRoleNameInvalid,
		},
		{
			name:        "space not allowed after normalization",
			input:       "Admin Smith",
			sentinelErr: ErrRoleNameInvalid,
		},
		{
			name:        "at-sign not allowed",
			input:       "admin@example",
			sentinelErr: ErrRoleNameInvalid,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseRoleName(tc.input)
			if tc.sentinelErr == nil {
				require.NoError(t, err)
				require.Equal(t, tc.wantValue, got)
				return
			}
			require.Error(t, err)
			require.True(t, errors.Is(err, tc.sentinelErr), "got %v", err)
		})
	}
}
