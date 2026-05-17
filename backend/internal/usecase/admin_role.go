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
	Create(ctx context.Context, name string) (CreateRoleOutcome, error)
	Update(ctx context.Context, id, name string) (UpdateRoleOutcome, error)
	Delete(ctx context.Context, id string) error
}

// CreateRoleOutcome is the result of adminRoleUsecase.Create. Exactly one of
// Role or Validation is non-nil on a nil-error return: a successful insert
// carries the new Role; a name that fails normalisation/validation or
// collides with an existing role surfaces via Validation so the resolver
// maps it to the CreateRoleResult union's InputValidationError variant.
type CreateRoleOutcome struct {
	Role       *domain.Role
	Validation *InputValidationInfo
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
// before reaching the repository. Validation failures and repository-level
// classified failures (duplicate name) surface via outcome.Validation so the
// resolver maps them to the CreateRoleResult union's InputValidationError
// variant; cancellation and infrastructure errors surface via the error
// return.
func (u *adminRoleUsecase) Create(ctx context.Context, name string) (CreateRoleOutcome, error) {
	if err := u.requireAdmin(ctx); err != nil {
		return CreateRoleOutcome{}, err
	}
	normalized, info, err := normalizeAndValidateRoleName(name)
	if err != nil {
		return CreateRoleOutcome{}, err
	}
	if info != nil {
		return CreateRoleOutcome{Validation: info}, nil
	}
	role, err := u.roles.Create(ctx, normalized)
	if err != nil {
		info, perr := mapAdminRoleError(err, "name", "usecase: admin role create")
		if perr != nil {
			return CreateRoleOutcome{}, perr
		}
		return CreateRoleOutcome{Validation: info}, nil
	}
	return CreateRoleOutcome{Role: role}, nil
}

// normalizeAndValidateRoleName wraps validateRoleName for outcome-bearing
// callers: it returns the canonical name on success, or an
// InputValidationInfo (with empty canonical name) on a validation failure.
// Non-validation errors are not expected from validateRoleName today; if
// one ever appears, it propagates via the error return so the caller can
// surface it as INTERNAL.
func normalizeAndValidateRoleName(name string) (string, *InputValidationInfo, error) {
	normalized, err := validateRoleName(name)
	if err != nil {
		info, perr := liftValidationErr(err)
		if perr != nil {
			return "", nil, perr
		}
		return "", info, nil
	}
	return normalized, nil, nil
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
		return UpdateRoleOutcome{}, lowerValidationInfo(mapAdminRoleError(err, "id", "usecase: admin role update: find"))
	}
	if isSystemRole(existing.Name) {
		return UpdateRoleOutcome{SystemRoleConflict: &SystemRoleConflictInfo{
			ID:   existing.ID,
			Name: existing.Name,
		}}, nil
	}

	role, err := u.roles.Update(ctx, id, normalized)
	if err != nil {
		return UpdateRoleOutcome{}, lowerValidationInfo(mapAdminRoleError(err, "id", "usecase: admin role update"))
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
		return lowerValidationInfo(mapAdminRoleError(err, "id", "usecase: admin role delete: find"))
	}
	if isSystemRole(existing.Name) {
		return ucerr.NewForbiddenError(fmt.Sprintf("cannot delete system role %q", existing.Name))
	}

	if err := u.roles.Delete(ctx, id); err != nil {
		return lowerValidationInfo(mapAdminRoleError(err, "id", "usecase: admin role delete"))
	}
	return nil
}

// mapAdminRoleError classifies the role-repository sentinel set into either
// input-validation data (first slot non-nil) or a propagating error (second
// slot non-nil). The shape mirrors mapRoleAssignmentError so promoted
// outcome-bearing callers can route validation refusals into outcome data
// uniformly:
//   - repository.ErrRoleNotFound            -> InputValidationInfo{Field: notFoundField}
//   - repository.ErrRoleDuplicate           -> InputValidationInfo{Field: "name"}
//   - context.Canceled / DeadlineExceeded   -> passthrough via error
//   - default                               -> eris.Wrap(err, wrap) via error
//
// The notFoundField argument lets callers name the request field that was bad
// (e.g. "id" for Update / Delete / Get-from-Update) without hardcoding a
// single field name here. Forbidden conditions are constructed at the call
// sites, not via this mapper.
//
// Unpromoted callers (Update, Delete) wrap the first-slot InputValidationInfo
// back into a *ucerr.ValidationError via lowerValidationInfo so their
// error-returning signatures stay intact.
func mapAdminRoleError(err error, notFoundField, wrap string) (*InputValidationInfo, error) {
	switch {
	case errors.Is(err, repository.ErrRoleNotFound):
		return &InputValidationInfo{Field: notFoundField, Message: "role not found"}, nil
	case errors.Is(err, repository.ErrRoleDuplicate):
		return &InputValidationInfo{Field: "name", Message: "role name already exists"}, nil
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return nil, err
	default:
		return nil, eris.Wrap(err, wrap)
	}
}

// lowerValidationInfo is the inverse of liftValidationErr for unpromoted
// callers of mapAdminRoleError (adminRoleUsecase.Update / Delete) that keep
// the legacy error-returning signature. Given the (info, err) tuple returned
// by mapAdminRoleError, it returns a single error: the propagating error if
// non-nil, otherwise a *ucerr.ValidationError reconstructed from the info,
// otherwise nil.
func lowerValidationInfo(info *InputValidationInfo, err error) error {
	if err != nil {
		return err
	}
	if info != nil {
		return ucerr.NewValidationError(info.Field, info.Message)
	}
	return nil
}
