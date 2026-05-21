package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/rotisserie/eris"

	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/usecase/ucerr"
)

// adminUserMaxPageSize is the user-facing cap on AdminUser.List page size.
// Mirrors the repository-level maxUserPageSize (100). The asymmetry between
// this value and userPageCap (101) at the repository layer enables the
// usecase-level "+1 fetch" trick without truncating a maximum-sized page.
const adminUserMaxPageSize = 100

// AdminUserConnection is the usecase-level Relay-style page result for
// AdminUser.List. The resolver wraps it into model.UserConnection.
type AdminUserConnection struct {
	Edges      []AdminUserEdge
	PageInfo   PageInfo
	TotalCount int64
}

// AdminUserEdge pairs a node with its opaque cursor (currently the user UUID).
type AdminUserEdge struct {
	Cursor string
	Node   *domain.User
}

// PageInfo mirrors the GraphQL PageInfo type. The pointer-typed cursors
// preserve "absent" semantics for the empty-page case.
type PageInfo struct {
	HasNextPage     bool
	HasPreviousPage bool
	StartCursor     *string
	EndCursor       *string
}

// AdminUpdateUserInput captures the patch fields for AdminUser.Update. nil
// means "leave unchanged"; non-nil with empty string means "explicit clear"
// for Bio. DisplayName must be 1-50 grapheme clusters when non-nil — empty
// strings are not accepted because there is no valid empty display name.
type AdminUpdateUserInput struct {
	DisplayName *string
	Bio         *string
}

// InputValidationInfo carries an input-validation failure as a typed value
// (not as an error). The resolver maps it to model.InputValidationError.
// Lives at package usecase so all promoted outcome-returning methods can
// share the carrier.
type InputValidationInfo struct {
	Field   string
	Message string
}

// NewInputValidationInfo constructs an InputValidationInfo. Panics on an
// empty field for the same reason as ucerr.NewValidationError: an empty
// field produces extensions.field == "" on the wire, which the frontend
// cannot render against any input.
func NewInputValidationInfo(field, message string) *InputValidationInfo {
	if field == "" {
		panic("usecase.NewInputValidationInfo: field must be non-empty")
	}
	return &InputValidationInfo{Field: field, Message: message}
}

// AssignRoleOutcome is the result of adminUserUsecase.AssignRole. Exactly one
// of User or Validation is non-nil on a nil-error return.
type AssignRoleOutcome struct {
	User       *domain.User
	Validation *InputValidationInfo
}

// RevokeRoleOutcome is the result of adminUserUsecase.RevokeRole. Exactly one
// of the three variant fields is the active slot on a nil-error return:
//   - User: happy path
//   - Validation: input validation refusal
//   - CannotRevokeOwnAdmin: domain invariant refusal (self-demotion of admin)
type RevokeRoleOutcome struct {
	User                 *domain.User
	Validation           *InputValidationInfo
	CannotRevokeOwnAdmin bool
}

// AdminUpdateUserOutcome is the result of adminUserUsecase.Update. Exactly one
// of User or Validation is non-nil on a nil-error return.
type AdminUpdateUserOutcome struct {
	User       *domain.User
	Validation *InputValidationInfo
}

// AdminUserUsecase is the admin-only user management surface.
type AdminUserUsecase interface {
	List(
		ctx context.Context,
		first, last *int,
		after, before, search *string,
	) (*AdminUserConnection, error)
	Get(ctx context.Context, id string) (*domain.User, error)
	Update(ctx context.Context, id string, input AdminUpdateUserInput) (AdminUpdateUserOutcome, error)
	AssignRole(ctx context.Context, userID, roleID string) (AssignRoleOutcome, error)
	RevokeRole(ctx context.Context, userID, roleID string) (RevokeRoleOutcome, error)
}

// adminUserRepository is the subset of repository.UserRepository the
// AdminUser usecase consumes. Declaring a narrow interface keeps the
// usecase test scaffolding small.
type adminUserRepository interface {
	FindByID(ctx context.Context, id string) (*domain.User, error)
	Update(ctx context.Context, id string, patch repository.UserUpdate) (*domain.User, error)
	ListPage(
		ctx context.Context,
		after, before *string,
		first, last int,
		search *string,
	) ([]*domain.User, int64, error)
}

// adminRoleRepository is the subset of repository.RoleRepository used by the
// AdminUser usecase (CRUD lookup only).
type adminRoleRepository interface {
	FindByIDs(ctx context.Context, ids []string) (map[string]*domain.Role, error)
}

// adminUserRoleRepository is the subset of repository.UserRoleRepository used
// by the AdminUser usecase (membership write operations).
type adminUserRoleRepository interface {
	AssignToUser(ctx context.Context, userID, roleID string) error
	RevokeFromUser(ctx context.Context, userID, roleID string) error
}

// adminUserUsecase wires the admin gate, the user repository, the role
// repository, and the user-role repository behind the admin-only management API.
type adminUserUsecase struct {
	users     adminUserRepository
	roles     adminRoleRepository     // FindByIDs (self-demotion guard in RevokeRole)
	userRoles adminUserRoleRepository // AssignToUser, RevokeFromUser
	adminGate *AdminGate
	logger    *slog.Logger
}

// NewAdminUser constructs an AdminUserUsecase. Pass production
// repository.UserRepository / repository.RoleRepository /
// repository.UserRoleRepository implementations; tests may pass narrower stubs
// that satisfy the package-private interfaces.
func NewAdminUser(
	users repository.UserRepository,
	roles repository.RoleRepository,
	userRoles repository.UserRoleRepository,
	adminGate *AdminGate,
	logger *slog.Logger,
) AdminUserUsecase {
	if adminGate == nil {
		panic("usecase: admin user: adminGate is required")
	}
	if logger == nil {
		panic("usecase: admin user: logger is required")
	}
	return &adminUserUsecase{users: users, roles: roles, userRoles: userRoles, adminGate: adminGate, logger: logger}
}

// NewAdminUserWithDeps is the test-time constructor that accepts the narrow
// interface types. Production code must use NewAdminUser.
func NewAdminUserWithDeps(
	users adminUserRepository,
	roles adminRoleRepository,
	userRoles adminUserRoleRepository,
	adminGate *AdminGate,
	logger *slog.Logger,
) AdminUserUsecase {
	if adminGate == nil {
		panic("usecase: admin user: adminGate is required")
	}
	if logger == nil {
		panic("usecase: admin user: logger is required")
	}
	return &adminUserUsecase{users: users, roles: roles, userRoles: userRoles, adminGate: adminGate, logger: logger}
}

// List paginates the users table with Relay-style cursors. Forward paging
// uses (first, after); backward uses (last, before). The two are mutually
// exclusive. Default page size is adminUserMaxPageSize (100); the same
// value is the absolute cap for either direction.
//
// Cursor-direction cross-validation: per the Relay spec, after pairs with
// first (forward) and before pairs with last (backward). Mixing them or
// supplying both cursors at once is rejected with BAD_USER_INPUT before the
// repository is touched, so callers never get a silently re-interpreted
// page boundary.
func (u *adminUserUsecase) List(
	ctx context.Context,
	first, last *int,
	after, before, search *string,
) (*AdminUserConnection, error) {
	if _, err := u.adminGate.Require(ctx, "usecase: admin user: check admin"); err != nil {
		return nil, err
	}

	// Relay argument coherence: after pairs with first (forward) and before
	// pairs with last (backward). A cursor without its companion count is also
	// rejected — page size and direction would be unresolvable.
	if err := validateRelayArgs(first, last, after, before); err != nil {
		return nil, err
	}

	wantFirst, wantLast, err := resolveAdminPageSize(first, last)
	if err != nil {
		return nil, err
	}

	// Request one extra row to detect whether another page exists. Trim the
	// extra row before returning to the caller.
	repoFirst := wantFirst
	repoLast := wantLast
	if repoFirst > 0 {
		repoFirst++
	}
	if repoLast > 0 {
		repoLast++
	}

	users, total, err := u.users.ListPage(ctx, after, before, repoFirst, repoLast, search)
	if err != nil {
		if errors.Is(err, repository.ErrCursorNotFound) {
			field := "after"
			if after == nil && before != nil {
				field = "before"
			}
			return nil, ucerr.NewValidationError(field, "cursor not found")
		}
		if isContextDone(err) {
			return nil, err
		}
		return nil, eris.Wrap(err, "usecase: admin user list")
	}

	out := &AdminUserConnection{TotalCount: total}
	switch {
	case wantFirst > 0:
		users, out.PageInfo.HasNextPage = TrimAndDetect(users, wantFirst)
		out.PageInfo.HasPreviousPage = after != nil
	case wantLast > 0:
		users, out.PageInfo.HasPreviousPage = TrimAndDetectBackward(users, wantLast)
		out.PageInfo.HasNextPage = before != nil
	}

	out.Edges = make([]AdminUserEdge, len(users))
	for i, user := range users {
		out.Edges[i] = AdminUserEdge{Cursor: user.ID, Node: user}
	}
	if len(users) > 0 {
		start := users[0].ID
		end := users[len(users)-1].ID
		out.PageInfo.StartCursor = &start
		out.PageInfo.EndCursor = &end
	}
	return out, nil
}

// Get returns a single user by id. Missing rows resolve to (nil, nil) so the
// resolver renders the GraphQL field as null without erroring.
func (u *adminUserUsecase) Get(ctx context.Context, id string) (*domain.User, error) {
	if _, err := u.adminGate.Require(ctx, "usecase: admin user: check admin"); err != nil {
		return nil, err
	}
	user, err := u.users.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil
		}
		if isContextDone(err) {
			return nil, err
		}
		return nil, eris.Wrap(err, "usecase: admin user get")
	}
	return user, nil
}

// Update applies the patch to the user identified by id. Validation mirrors
// UserUsecase.UpdateUser: display_name must be 1-50 grapheme clusters when
// non-nil, bio must be at most 500 grapheme clusters when non-nil, and an
// explicit empty bio clears the column. Validation failures surface via the
// outcome's Validation slot ("errors as data") so the resolver maps them to
// the AdminUpdateUserResult union's InputValidationError variant.
func (u *adminUserUsecase) Update(ctx context.Context, id string, input AdminUpdateUserInput) (AdminUpdateUserOutcome, error) {
	if _, err := u.adminGate.Require(ctx, "usecase: admin user: check admin"); err != nil {
		return AdminUpdateUserOutcome{}, err
	}

	patch := repository.UserUpdate{}
	if input.DisplayName != nil {
		dn, err := domain.ParseDisplayName(*input.DisplayName)
		if err != nil {
			info, perr := liftValidationErr(translateDisplayNameErr(err))
			if perr != nil {
				return AdminUpdateUserOutcome{}, perr
			}
			return AdminUpdateUserOutcome{Validation: info}, nil
		}
		s := string(dn)
		patch.DisplayName = &s
	}
	if input.Bio != nil {
		bio, err := domain.ParseBio(input.Bio)
		if err != nil {
			info, perr := liftValidationErr(translateBioErr(err))
			if perr != nil {
				return AdminUpdateUserOutcome{}, perr
			}
			return AdminUpdateUserOutcome{Validation: info}, nil
		}
		patch.Bio = bio.Ptr()
	}

	user, err := u.users.Update(ctx, id, patch)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return AdminUpdateUserOutcome{
				Validation: NewInputValidationInfo("id", "user not found"),
			}, nil
		}
		if isContextDone(err) {
			return AdminUpdateUserOutcome{}, err
		}
		return AdminUpdateUserOutcome{}, eris.Wrap(err, "usecase: admin user update")
	}
	return AdminUpdateUserOutcome{User: user}, nil
}

// AssignRole grants roleID to userID. Idempotent at the repository layer:
// calling twice with the same ids is a no-op the second time.
//
// Error mapping: distinguishes user-missing from role-missing via the
// repository's ErrUserNotFound / ErrRoleNotFound sentinels so the
// InputValidationError variant carries the field name so the frontend can
// attach the message next to the offending input (userId vs roleId). The
// generic ErrNotFound branch is kept as a fallback for any future repository
// implementation that surfaces only the legacy sentinel. Input-validation
// failures surface via outcome.Validation; infrastructure and cancellation
// errors surface via the error return.
func (u *adminUserUsecase) AssignRole(ctx context.Context, userID, roleID string) (AssignRoleOutcome, error) {
	if _, err := u.adminGate.Require(ctx, "usecase: admin user: check admin"); err != nil {
		return AssignRoleOutcome{}, err
	}
	if err := u.userRoles.AssignToUser(ctx, userID, roleID); err != nil {
		info, perr := mapRoleAssignmentError(err, "usecase: admin user assign role")
		if perr != nil {
			return AssignRoleOutcome{}, perr
		}
		return AssignRoleOutcome{Validation: info}, nil
	}
	user, err := u.refetchUser(ctx, userID, "usecase: admin user assign role: refetch")
	if err != nil {
		return AssignRoleOutcome{}, err
	}
	return AssignRoleOutcome{User: user}, nil
}

// RevokeRole removes roleID from userID. Self-demotion of the admin role is
// surfaced via outcome.CannotRevokeOwnAdmin so the resolver maps it to the
// CannotRevokeOwnAdminRoleError union variant (domain invariant as data,
// not as an error). Idempotent at the repository layer otherwise.
func (u *adminUserUsecase) RevokeRole(ctx context.Context, userID, roleID string) (RevokeRoleOutcome, error) {
	callerID, err := u.adminGate.Require(ctx, "usecase: admin user: check admin")
	if err != nil {
		return RevokeRoleOutcome{}, err
	}

	// Self-demotion guard: only blocks revoking the *admin* role from the
	// caller themselves. Look up the role first so the check is deterministic
	// even when the caller passes a stale roleID.
	if userID == callerID {
		roles, err := u.roles.FindByIDs(ctx, []string{roleID})
		if err != nil {
			if isContextDone(err) {
				return RevokeRoleOutcome{}, err
			}
			return RevokeRoleOutcome{}, eris.Wrap(err, "usecase: admin user revoke role: lookup role")
		}
		if role, ok := roles[roleID]; ok && role.Name == domain.AdminRoleName {
			return RevokeRoleOutcome{CannotRevokeOwnAdmin: true}, nil
		}
	}

	if err := u.userRoles.RevokeFromUser(ctx, userID, roleID); err != nil {
		info, perr := mapRoleAssignmentError(err, "usecase: admin user revoke role")
		if perr != nil {
			return RevokeRoleOutcome{}, perr
		}
		return RevokeRoleOutcome{Validation: info}, nil
	}
	user, err := u.refetchUser(ctx, userID, "usecase: admin user revoke role: refetch")
	if err != nil {
		return RevokeRoleOutcome{}, err
	}
	return RevokeRoleOutcome{User: user}, nil
}

// mapRoleAssignmentError classifies the sentinel set returned by
// userRoles.AssignToUser / RevokeFromUser into either input-validation data
// (returned via the first slot, with second slot nil) or a propagating error
// (returned via the second slot, with first slot nil). Specific sentinels
// are matched before the legacy ErrNotFound fallback because both
// ErrUserNotFound and ErrRoleNotFound also satisfy errors.Is(_, ErrNotFound).
// Context cancellation passes through unwrapped.
func mapRoleAssignmentError(err error, wrap string) (*InputValidationInfo, error) {
	switch {
	case errors.Is(err, repository.ErrUserNotFound):
		return NewInputValidationInfo("userId", "user not found"), nil
	case errors.Is(err, repository.ErrRoleNotFound):
		return NewInputValidationInfo("roleId", "role not found"), nil
	case errors.Is(err, repository.ErrNotFound):
		return NewInputValidationInfo("userId", "user or role not found"), nil
	case isContextDone(err):
		return nil, err
	default:
		return nil, eris.Wrap(err, wrap)
	}
}

// liftValidationErr bridges a validator that returns error into an outcome-
// bearing call site. *ucerr.ValidationError values are unwrapped into an
// InputValidationInfo carrier (first slot); any other error is passed through
// unchanged (second slot). nil maps to (nil, nil). The shape lets call sites
// in promoted methods uniformly route validation failures into outcome data
// without having to re-classify each validator's return type.
func liftValidationErr(err error) (*InputValidationInfo, error) {
	if err == nil {
		return nil, nil
	}
	if ve, ok := errors.AsType[*ucerr.ValidationError](err); ok {
		return NewInputValidationInfo(ve.Field, ve.Message), nil
	}
	return nil, err
}

// refetchUser loads the user after a mutation so callers see a fresh row
// (e.g. with the trigger-refreshed updated_at). A missing row after a
// successful mutation is unusual; surface it as INTERNAL with the supplied
// context. The original ErrNotFound is wrapped (not replaced) so the chain
// stays intact for errors.Is checks downstream and so the eris error_chain
// log entry preserves the originating sentinel.
func (u *adminUserUsecase) refetchUser(ctx context.Context, id, wrap string) (*domain.User, error) {
	user, err := u.users.FindByID(ctx, id)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			return nil, eris.Wrapf(err, "%s: user disappeared", wrap)
		case isContextDone(err):
			return nil, err
		default:
			return nil, eris.Wrap(err, wrap)
		}
	}
	return user, nil
}

// resolveAdminPageSize enforces the (first XOR last) constraint and clamps
// each value to [0, adminUserMaxPageSize]. When both are nil, defaults to
// (adminUserMaxPageSize, 0) — the requirement to default forward paging at
// the documented maximum keeps single-page admin queries simple.
func resolveAdminPageSize(first, last *int) (int, int, error) {
	if first != nil && last != nil {
		return 0, 0, ucerr.NewValidationError("first", "specify either first or last")
	}
	if first == nil && last == nil {
		return adminUserMaxPageSize, 0, nil
	}
	check := func(field string, v int) error {
		if v < 0 {
			return ucerr.NewValidationError(field, fmt.Sprintf("%s must be >= 0", field))
		}
		if v > adminUserMaxPageSize {
			return ucerr.NewValidationError(field, fmt.Sprintf("%s must be <= %d", field, adminUserMaxPageSize))
		}
		return nil
	}
	if first != nil {
		if err := check("first", *first); err != nil {
			return 0, 0, err
		}
		return *first, 0, nil
	}
	if err := check("last", *last); err != nil {
		return 0, 0, err
	}
	return 0, *last, nil
}
