package usecase

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/rivo/uniseg"
	"github.com/rotisserie/eris"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/gqlerr"
	"backend/internal/repository"
)

// roleNameMin and roleNameMax bound the post-normalisation grapheme-cluster
// length of a role name. They mirror the displayName bounds used elsewhere in
// the package; the lower bound rejects whitespace-only input that collapses
// to "" after TrimSpace, the upper bound matches the column constraint.
const (
	roleNameMin = 1
	roleNameMax = 50
)

// roleNamePattern restricts the post-normalisation role name to lowercase
// ASCII letters, digits, underscore, and hyphen. The set is deliberately
// narrow so role names round-trip cleanly through URLs, log lines, and the
// frontend without escaping. The regex anchors both ends so any extra
// character (whitespace, punctuation, non-ASCII) rejects the whole input.
var roleNamePattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

// AdminRoleUsecase is the admin-only role CRUD surface. The methods are kept
// separate from AdminUserUsecase because the responsibilities differ — user
// management and role management share an auth gate but no business state.
type AdminRoleUsecase interface {
	List(ctx context.Context) ([]*domain.Role, error)
	Get(ctx context.Context, id string) (*domain.Role, error)
	Create(ctx context.Context, name string) (*domain.Role, error)
	Update(ctx context.Context, id, name string) (*domain.Role, error)
	Delete(ctx context.Context, id string) error
}

// adminRoleRepoForCRUD is the narrow repository surface consumed by the
// AdminRole usecase. It is intentionally a separate interface from
// adminRoleRepository (used by adminUserUsecase) — the two have no methods in
// common in production callers, and conflating them would force test doubles
// for AdminRole to also implement assignment plumbing they never exercise.
type adminRoleRepoForCRUD interface {
	FindByID(ctx context.Context, id string) (*domain.Role, error)
	Create(ctx context.Context, name string) (*domain.Role, error)
	Update(ctx context.Context, id, name string) (*domain.Role, error)
	Delete(ctx context.Context, id string) error
	ListAll(ctx context.Context) ([]*domain.Role, error)
}

// adminRoleUsecase wires the role repository and the admin checker behind the
// admin-only role-management API.
type adminRoleUsecase struct {
	roles adminRoleRepoForCRUD
	auth  AdminChecker
}

// NewAdminRole constructs an AdminRoleUsecase from the production repository
// and auth service. Tests should prefer NewAdminRoleWithDeps to inject narrow
// stubs.
func NewAdminRole(roles repository.RoleRepository, authSvc *auth.Service) AdminRoleUsecase {
	return &adminRoleUsecase{roles: roles, auth: authSvc}
}

// NewAdminRoleWithDeps is the test-time constructor that accepts the narrow
// interface types. Production code must use NewAdminRole.
func NewAdminRoleWithDeps(roles adminRoleRepoForCRUD, authSvc AdminChecker) AdminRoleUsecase {
	return &adminRoleUsecase{roles: roles, auth: authSvc}
}

// requireAdmin centralises the auth gate every method shares. It mirrors the
// adminUserUsecase.requireAdmin shape so the two surfaces use the same
// FORBIDDEN / UNAUTHENTICATED / CANCELLED / INTERNAL classification.
func (u *adminRoleUsecase) requireAdmin(ctx context.Context) error {
	caller := auth.UserFrom(ctx)
	if caller == nil || caller.Sub == "" {
		return gqlerr.Unauthenticated()
	}
	if u.auth == nil {
		return gqlerr.Internal(ctx, eris.New("usecase: admin role: admin checker not configured"))
	}
	isAdmin, err := u.auth.IsAdmin(ctx, caller.Sub)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return gqlerr.Cancelled(ctx, err)
		}
		return gqlerr.Internal(ctx, eris.Wrap(err, "usecase: admin role: check admin"))
	}
	if !isAdmin {
		return gqlerr.NewForbidden("admin only")
	}
	return nil
}

// validateRoleName trims, lowercases, and validates a user-supplied role
// name. It returns the canonical form on success so the caller passes a
// consistent value to the repository (defence-in-depth — the repository also
// normalises). The grapheme-cluster check matches the displayName surface so
// a multi-codepoint emoji counts as one character; the regex check is on the
// post-normalisation byte form, which for ASCII-only names is equivalent.
func validateRoleName(name string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(name))
	n := uniseg.GraphemeClusterCount(normalized)
	if n < roleNameMin {
		return "", gqlerr.BadUserInput("name", "name is required")
	}
	if n > roleNameMax {
		return "", gqlerr.BadUserInput("name",
			fmt.Sprintf("name must be at most %d characters", roleNameMax))
	}
	if !roleNamePattern.MatchString(normalized) {
		return "", gqlerr.BadUserInput("name",
			"name must contain only lowercase letters, digits, '_' or '-'")
	}
	return normalized, nil
}

// List returns every role in the system. Admin-only.
func (u *adminRoleUsecase) List(ctx context.Context) ([]*domain.Role, error) {
	if err := u.requireAdmin(ctx); err != nil {
		return nil, err
	}
	roles, err := u.roles.ListAll(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, gqlerr.Cancelled(ctx, err)
		}
		return nil, gqlerr.Internal(ctx, eris.Wrap(err, "usecase: admin role list"))
	}
	return roles, nil
}

// Get returns a single role by id. Missing rows resolve to (nil, nil) so the
// resolver renders the GraphQL field as null without erroring — Query.role(id)
// is nullable in the schema for this reason.
func (u *adminRoleUsecase) Get(ctx context.Context, id string) (*domain.Role, error) {
	if err := u.requireAdmin(ctx); err != nil {
		return nil, err
	}
	role, err := u.roles.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrRoleNotFound) {
			return nil, nil
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, gqlerr.Cancelled(ctx, err)
		}
		return nil, gqlerr.Internal(ctx, eris.Wrap(err, "usecase: admin role get"))
	}
	return role, nil
}

// Create inserts a new role with the supplied name. The name is normalised
// (trimmed, lowercased) and validated against the role-name character set
// before reaching the repository.
func (u *adminRoleUsecase) Create(ctx context.Context, name string) (*domain.Role, error) {
	if err := u.requireAdmin(ctx); err != nil {
		return nil, err
	}
	normalized, err := validateRoleName(name)
	if err != nil {
		return nil, err
	}
	role, err := u.roles.Create(ctx, normalized)
	if err != nil {
		return nil, mapAdminRoleError(ctx, err, "name", "usecase: admin role create")
	}
	return role, nil
}

// Update renames an existing role. The system "admin" role is renaming-locked:
// renaming it would break the auth.Service.IsAdmin lookup that hardcodes the
// literal "admin" string. The lookup runs before the repository update so the
// FORBIDDEN response does not depend on the new name passing validation.
//
// TOCTOU note: between FindByID and roles.Update another admin can delete the
// role; the resulting ErrRoleNotFound from Update is mapped back to
// BAD_USER_INPUT(field=id) rather than INTERNAL.
func (u *adminRoleUsecase) Update(ctx context.Context, id, name string) (*domain.Role, error) {
	if err := u.requireAdmin(ctx); err != nil {
		return nil, err
	}
	normalized, err := validateRoleName(name)
	if err != nil {
		return nil, err
	}

	existing, err := u.roles.FindByID(ctx, id)
	if err != nil {
		return nil, mapAdminRoleError(ctx, err, "id", "usecase: admin role update: find")
	}
	if existing.Name == adminRoleName {
		return nil, gqlerr.NewForbidden("cannot rename system role 'admin'")
	}

	role, err := u.roles.Update(ctx, id, normalized)
	if err != nil {
		return nil, mapAdminRoleError(ctx, err, "id", "usecase: admin role update")
	}
	return role, nil
}

// Delete removes a role by id. The system "admin" role is delete-locked: a
// missing admin role would lock every operator out of the management API. The
// guard is enforced by name (not by id) because admin-role provisioning is
// idempotent against the name in the migrations.
//
// TOCTOU note: between FindByID and roles.Delete another admin can delete the
// role; the resulting ErrRoleNotFound from Delete is mapped back to
// BAD_USER_INPUT(field=id) rather than INTERNAL.
func (u *adminRoleUsecase) Delete(ctx context.Context, id string) error {
	if err := u.requireAdmin(ctx); err != nil {
		return err
	}

	existing, err := u.roles.FindByID(ctx, id)
	if err != nil {
		return mapAdminRoleError(ctx, err, "id", "usecase: admin role delete: find")
	}
	if existing.Name == adminRoleName {
		return gqlerr.NewForbidden("cannot delete system role 'admin'")
	}

	if err := u.roles.Delete(ctx, id); err != nil {
		return mapAdminRoleError(ctx, err, "id", "usecase: admin role delete")
	}
	return nil
}

// mapAdminRoleError translates the role-repository sentinel set into the
// typed gqlerr surface. The notFoundField argument lets callers say "the id
// in this request was bad" (Update / Delete / Get-from-Update) without
// hardcoding a single field name.
//
// ErrRoleDuplicate always maps to field="name" — the duplicate condition is
// always on the name column, regardless of which method surfaced it.
func mapAdminRoleError(ctx context.Context, err error, notFoundField, wrap string) error {
	switch {
	case errors.Is(err, repository.ErrRoleNotFound):
		return gqlerr.BadUserInput(notFoundField, "role not found")
	case errors.Is(err, repository.ErrRoleDuplicate):
		return gqlerr.BadUserInput("name", "role name already exists")
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return gqlerr.Cancelled(ctx, err)
	default:
		return gqlerr.Internal(ctx, eris.Wrap(err, wrap))
	}
}
