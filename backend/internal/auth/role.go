package auth

import (
	"context"

	"github.com/rotisserie/eris"

	"backend/internal/domain"
)

// roleChecker is the narrow interface auth.Service requires from the repository
// layer. Only HasRole is needed; callers may pass any repository.UserRoleRepository.
type roleChecker interface {
	HasRole(ctx context.Context, userID string, roleName domain.RoleName) (bool, error)
}

// Service centralises authorisation helpers that depend on durable role state.
// Construct once at boot and pass into resolvers / usecases that need to gate
// on role membership. Keeping this struct distinct from the request-scoped
// AuthUser lets us inject a mock in tests.
type Service struct {
	userRoles roleChecker
}

// NewService wires the auth.Service against a role-checking repository.
func NewService(userRoles roleChecker) *Service {
	return &Service{userRoles: userRoles}
}

// IsAdmin reports whether userID holds the admin role (domain.AdminRoleName).
// The role name is defined in the domain package to keep usecases from drifting to bespoke names.
func (s *Service) IsAdmin(ctx context.Context, userID string) (bool, error) {
	ok, err := s.userRoles.HasRole(ctx, userID, domain.AdminRoleName)
	if err != nil {
		return false, eris.Wrap(err, "auth: check admin role")
	}
	return ok, nil
}
