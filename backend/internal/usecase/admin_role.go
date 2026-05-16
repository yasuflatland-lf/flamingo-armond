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
	"backend/internal/repository"
	"backend/internal/usecase/ucerr"
)

// roleNameMin / roleNameMax bound the post-normalisation grapheme-cluster
// length of a role name. The lower bound rejects whitespace-only input that
// collapses to "" after TrimSpace; the upper bound matches the column constraint.
const (
	roleNameMin = 1
	roleNameMax = 50
)

// roleNamePattern restricts the post-normalisation role name to lowercase
// ASCII letters, digits, underscore, and hyphen. The set is deliberately narrow
// so role names round-trip cleanly through URLs, log lines, and the frontend
// without escaping.
var roleNamePattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

// AdminRoleUsecase is the admin-only role CRUD surface. Kept separate from
// AdminUserUsecase: the two share an auth gate but no business state.
type AdminRoleUsecase interface {
	List(ctx context.Context) ([]*domain.Role, error)
	Get(ctx context.Context, id string) (*domain.Role, error)
	Create(ctx context.Context, name string) (*domain.Role, error)
	Update(ctx context.Context, id, name string) (UpdateRoleOutcome, error)
	Delete(ctx context.Context, id string) error
}

// adminRoleRepoForCRUD is the narrow repository surface consumed by the
// AdminRole usecase. Kept separate from adminRoleRepository (used by
// adminUserUsecase) so test doubles for AdminRole are not forced to implement
// assignment plumbing they never exercise.
type adminRoleRepoForCRUD interface {
	FindByID(ctx context.Context, id string) (*domain.Role, error)
	Create(ctx context.Context, name string) (*domain.Role, error)
	Update(ctx context.Context, id, name string) (*domain.Role, error)
	Delete(ctx context.Context, id string) error
	ListAll(ctx context.Context) ([]*domain.Role, error)
}

type adminRoleUsecase struct {
	roles adminRoleRepoForCRUD
	auth  AdminChecker
}

// NewAdminRole is the production constructor. Tests should prefer
// NewAdminRoleWithDeps to inject narrow stubs.
func NewAdminRole(roles repository.RoleRepository, authSvc *auth.Service) AdminRoleUsecase {
	return &adminRoleUsecase{roles: roles, auth: authSvc}
}

// NewAdminRoleWithDeps accepts narrow interface types for tests; production
// code must use NewAdminRole.
func NewAdminRoleWithDeps(roles adminRoleRepoForCRUD, authSvc AdminChecker) AdminRoleUsecase {
	return &adminRoleUsecase{roles: roles, auth: authSvc}
}

// requireAdmin centralises the auth gate, mirroring adminUserUsecase.requireAdmin
// so the two surfaces share the same FORBIDDEN / UNAUTHENTICATED / CANCELLED /
// INTERNAL classification.
func (u *adminRoleUsecase) requireAdmin(ctx context.Context) error {
	caller := auth.UserFrom(ctx)
	if caller == nil || caller.Sub == "" {
		return ucerr.ErrUnauthenticated
	}
	if u.auth == nil {
		return eris.New("usecase: admin role: admin checker not configured")
	}
	isAdmin, err := u.auth.IsAdmin(ctx, caller.Sub)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return eris.Wrap(err, "usecase: admin role: check admin")
	}
	if !isAdmin {
		return ucerr.NewForbiddenError("admin only")
	}
	return nil
}

// validateRoleName trims, lowercases, and validates a user-supplied role name.
// Returns the canonical form on success so the caller passes a consistent value
// to the repository (defence-in-depth — the repository also normalises).
func validateRoleName(name string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(name))
	n := uniseg.GraphemeClusterCount(normalized)
	if n < roleNameMin {
		return "", ucerr.NewValidationError("name", "name is required")
	}
	if n > roleNameMax {
		return "", ucerr.NewValidationError("name", fmt.Sprintf("name must be at most %d characters", roleNameMax))
	}
	if !roleNamePattern.MatchString(normalized) {
		return "", ucerr.NewValidationError("name", "name must contain only lowercase letters, digits, '_' or '-'")
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
			return nil, err
		}
		return nil, eris.Wrap(err, "usecase: admin role list")
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
			return nil, err
		}
		return nil, eris.Wrap(err, "usecase: admin role get")
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
		return nil, mapAdminRoleError(err, "name", "usecase: admin role create")
	}
	return role, nil
}

// UpdateRoleOutcome is the result of admin_role.Update. Exactly one of Role
// or SystemRoleConflict is non-nil: a successful rename returns the updated
// Role, while a refusal to rename a system role ("admin", "general") returns
// a populated SystemRoleConflict and a nil error.
//
// The system-role guard is a domain invariant of the Role aggregate, not an
// authorisation failure: even an admin caller cannot rename "admin". Surfacing
// the refusal as data (rather than as an error) lets the resolver map it to
// the model.CannotModifySystemRoleError union variant, which carries the
// offending role's id and name for client-side rendering without a second
// fetch.
//
// Validation failures, unauthenticated callers, and repository errors stay in
// the second return value (error) so the resolver wraps them via
// gqlerr.FromUsecaseError into the wire-format GraphQL error.
type UpdateRoleOutcome struct {
	// Role is the renamed role on the happy path. Non-nil iff
	// SystemRoleConflict is nil.
	Role *domain.Role
	// SystemRoleConflict carries the offending system role's identity when
	// the rename was refused by the system-role guard. Non-nil iff Role is nil.
	SystemRoleConflict *SystemRoleConflictInfo
}

// SystemRoleConflictInfo identifies the system role that an Update call
// refused to rename.
type SystemRoleConflictInfo struct {
	ID   string
	Name string
}

// Update renames an existing role and returns an UpdateRoleOutcome that
// signals the system-role refusal as data (via outcome.SystemRoleConflict)
// rather than as an error. System roles ("admin", "general") are
// renaming-locked: renaming "admin" would break the auth.Service.IsAdmin
// lookup that hardcodes the literal string, and renaming "general" would
// break any deployment that relies on the literal name being present.
//
// TOCTOU: between FindByID and roles.Update another admin can delete the row;
// the resulting ErrRoleNotFound is mapped back to BAD_USER_INPUT(field=id)
// rather than INTERNAL.
func (u *adminRoleUsecase) Update(ctx context.Context, id, name string) (UpdateRoleOutcome, error) {
	if err := u.requireAdmin(ctx); err != nil {
		return UpdateRoleOutcome{}, err
	}
	normalized, err := validateRoleName(name)
	if err != nil {
		return UpdateRoleOutcome{}, err
	}

	existing, err := u.roles.FindByID(ctx, id)
	if err != nil {
		return UpdateRoleOutcome{}, mapAdminRoleError(err, "id", "usecase: admin role update: find")
	}
	if isSystemRole(existing.Name) {
		return UpdateRoleOutcome{SystemRoleConflict: &SystemRoleConflictInfo{
			ID:   existing.ID,
			Name: existing.Name,
		}}, nil
	}

	role, err := u.roles.Update(ctx, id, normalized)
	if err != nil {
		return UpdateRoleOutcome{}, mapAdminRoleError(err, "id", "usecase: admin role update")
	}
	return UpdateRoleOutcome{Role: role}, nil
}

// Delete removes a role by id. System roles ("admin", "general") are
// delete-locked: a missing admin role would lock every operator out of the
// management API, and a missing general role would orphan deployments that
// reference it by literal name. The guard matches by name (not by id)
// because system-role provisioning is idempotent against the name in the
// migrations.
//
// TOCTOU: between FindByID and roles.Delete another admin can delete the row;
// the resulting ErrRoleNotFound is mapped back to BAD_USER_INPUT(field=id)
// rather than INTERNAL.
func (u *adminRoleUsecase) Delete(ctx context.Context, id string) error {
	if err := u.requireAdmin(ctx); err != nil {
		return err
	}

	existing, err := u.roles.FindByID(ctx, id)
	if err != nil {
		return mapAdminRoleError(err, "id", "usecase: admin role delete: find")
	}
	if isSystemRole(existing.Name) {
		return ucerr.NewForbiddenError(fmt.Sprintf("cannot delete system role %q", existing.Name))
	}

	if err := u.roles.Delete(ctx, id); err != nil {
		return mapAdminRoleError(err, "id", "usecase: admin role delete")
	}
	return nil
}

// mapAdminRoleError classifies the role-repository sentinel set into the
// outcomes the resolver consumes via gqlerr.FromUsecaseError:
//   - repository.ErrRoleNotFound            -> *ucerr.ValidationError{Field: notFoundField}
//   - repository.ErrRoleDuplicate           -> *ucerr.ValidationError{Field: "name"}
//   - context.Canceled / DeadlineExceeded   -> passthrough
//   - default                               -> eris.Wrap(err, wrap) (opaque chain -> INTERNAL)
//
// The notFoundField argument lets callers name the request field that was bad
// (e.g. "id" for Update / Delete / Get-from-Update) without hardcoding a
// single field name here.
//
// Forbidden conditions are constructed at the call sites, not via this mapper.
func mapAdminRoleError(err error, notFoundField, wrap string) error {
	switch {
	case errors.Is(err, repository.ErrRoleNotFound):
		return ucerr.NewValidationError(notFoundField, "role not found")
	case errors.Is(err, repository.ErrRoleDuplicate):
		return ucerr.NewValidationError("name", "role name already exists")
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	default:
		return eris.Wrap(err, wrap)
	}
}
