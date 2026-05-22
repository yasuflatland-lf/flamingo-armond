package usecase

import (
	"errors"
	"testing"

	"backend/internal/auth"
	"backend/internal/usecase/ucerr"

	"github.com/stretchr/testify/require"
)

func TestRequireCallerSub(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		caller  *auth.AuthUser
		wantErr error
	}{
		{
			name:    "nil caller returns ErrUnauthenticated",
			caller:  nil,
			wantErr: ucerr.ErrUnauthenticated,
		},
		{
			name:    "empty Sub returns ErrUnauthenticated",
			caller:  &auth.AuthUser{Sub: ""},
			wantErr: ucerr.ErrUnauthenticated,
		},
		{
			name:    "valid Sub returns nil",
			caller:  &auth.AuthUser{Sub: "user-123"},
			wantErr: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := requireCallerSub(tc.caller)
			if tc.wantErr == nil {
				require.NoError(t, err)
			} else {
				require.True(t, errors.Is(err, tc.wantErr),
					"expected %v, got %v", tc.wantErr, err)
			}
		})
	}
}
