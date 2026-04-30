package auth_test

import (
	"context"
	"errors"
	"testing"

	"backend/internal/auth"
)

type stubUserRoles struct {
	hasRole bool
	err     error
	gotUID  string
	gotRole string
}

func (s *stubUserRoles) HasRole(_ context.Context, userID, roleName string) (bool, error) {
	s.gotUID = userID
	s.gotRole = roleName
	return s.hasRole, s.err
}

func TestServiceIsAdmin(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		stub    *stubUserRoles
		wantOK  bool
		wantErr bool
	}{
		{"admin returns true", &stubUserRoles{hasRole: true}, true, false},
		{"non-admin returns false", &stubUserRoles{hasRole: false}, false, false},
		{"db error wrapped", &stubUserRoles{err: errors.New("boom")}, false, true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := auth.NewService(tc.stub)
			ok, err := svc.IsAdmin(context.Background(), "user-1")
			if (err != nil) != tc.wantErr {
				t.Fatalf("err mismatch: got %v, wantErr=%v", err, tc.wantErr)
			}
			if ok != tc.wantOK {
				t.Fatalf("IsAdmin: got %v want %v", ok, tc.wantOK)
			}
			if !tc.wantErr {
				if tc.stub.gotUID != "user-1" {
					t.Errorf("uid passed through: got %q", tc.stub.gotUID)
				}
				if tc.stub.gotRole != "admin" {
					t.Errorf("role passed through: got %q", tc.stub.gotRole)
				}
			}
		})
	}
}
