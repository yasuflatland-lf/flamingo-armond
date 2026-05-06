package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rotisserie/eris"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/gqlerr"
	"backend/internal/repository"
)

// adminRoleName is the name of the role that grants admin privileges. It is
// hardcoded here for the self-demotion guard in RevokeRole. Keeping the
// literal in one place avoids drift from the same constant baked into
// auth.Service.IsAdmin.
const adminRoleName = "admin"

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

// AdminUserUsecase is the admin-only user management surface.
type AdminUserUsecase interface {
	List(
		ctx context.Context,
		first, last *int,
		after, before, search *string,
		roleID *string,
	) (*AdminUserConnection, error)
	Get(ctx context.Context, id string) (*domain.User, error)
	Update(ctx context.Context, id string, input AdminUpdateUserInput) (*domain.User, error)
	AssignRole(ctx context.Context, userID, roleID string) (*domain.User, error)
	RevokeRole(ctx context.Context, userID, roleID string) (*domain.User, error)
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
		roleID *string,
	) ([]*domain.User, int64, error)
}

// adminRoleRepository is the subset of repository.RoleRepository used by the
// AdminUser usecase.
type adminRoleRepository interface {
	FindByIDs(ctx context.Context, ids []string) (map[string]*domain.Role, error)
	AssignToUser(ctx context.Context, userID, roleID string) error
	RevokeFromUser(ctx context.Context, userID, roleID string) error
}

// adminUserUsecase wires the auth service, the user repository, and the role
// repository behind the admin-only management API.
type adminUserUsecase struct {
	users adminUserRepository
	roles adminRoleRepository
	auth  AdminChecker
}

// NewAdminUser constructs an AdminUserUsecase. Pass production
// repository.UserRepository / repository.RoleRepository implementations;
// tests may pass narrower stubs that satisfy the package-private
// adminUserRepository / adminRoleRepository interfaces.
func NewAdminUser(
	users repository.UserRepository,
	roles repository.RoleRepository,
	authSvc *auth.Service,
) AdminUserUsecase {
	return &adminUserUsecase{users: users, roles: roles, auth: authSvc}
}

// NewAdminUserWithDeps is the test-time constructor that accepts the narrow
// interface types. Production code must use NewAdminUser.
func NewAdminUserWithDeps(
	users adminUserRepository,
	roles adminRoleRepository,
	authSvc AdminChecker,
) AdminUserUsecase {
	return &adminUserUsecase{users: users, roles: roles, auth: authSvc}
}

// requireAdmin centralises the auth gate every method shares. It returns the
// caller's user id alongside any error; callers use the id for the
// self-demotion guard in RevokeRole.
func (u *adminUserUsecase) requireAdmin(ctx context.Context) (string, error) {
	caller := auth.UserFrom(ctx)
	if caller == nil || caller.Sub == "" {
		return "", gqlerr.Unauthenticated()
	}
	if u.auth == nil {
		return "", gqlerr.Internal(ctx, eris.New("usecase: admin user: admin checker not configured"))
	}
	isAdmin, err := u.auth.IsAdmin(ctx, caller.Sub)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return "", gqlerr.Cancelled(ctx, err)
		}
		return "", gqlerr.Internal(ctx, eris.Wrap(err, "usecase: admin user: check admin"))
	}
	if !isAdmin {
		return "", gqlerr.NewForbidden("admin only")
	}
	return caller.Sub, nil
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
	roleID *string,
) (*AdminUserConnection, error) {
	if _, err := u.requireAdmin(ctx); err != nil {
		return nil, err
	}

	if after != nil && before != nil {
		return nil, gqlerr.BadUserInput("after", "after and before are mutually exclusive")
	}
	if first != nil && *first > 0 && before != nil {
		return nil, gqlerr.BadUserInput("before", "before requires last, not first")
	}
	if last != nil && *last > 0 && after != nil {
		return nil, gqlerr.BadUserInput("after", "after requires first, not last")
	}
	// A cursor without its companion count is ambiguous: the server cannot
	// determine page size or direction. Reject early so the repository is
	// never called with an uninterpretable combination.
	if before != nil && (first == nil || *first <= 0) && (last == nil || *last <= 0) {
		return nil, gqlerr.BadUserInput("before", "before requires last")
	}
	if after != nil && (first == nil || *first <= 0) && (last == nil || *last <= 0) {
		return nil, gqlerr.BadUserInput("after", "after requires first")
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

	users, total, err := u.users.ListPage(ctx, after, before, repoFirst, repoLast, search, roleID)
	if err != nil {
		if errors.Is(err, repository.ErrCursorNotFound) {
			field := "after"
			if after == nil && before != nil {
				field = "before"
			}
			return nil, gqlerr.BadUserInput(field, "cursor not found")
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, gqlerr.Cancelled(ctx, err)
		}
		return nil, gqlerr.Internal(ctx, eris.Wrap(err, "usecase: admin user list"))
	}

	out := &AdminUserConnection{TotalCount: total}
	switch {
	case wantFirst > 0:
		if len(users) > wantFirst {
			out.PageInfo.HasNextPage = true
			users = users[:wantFirst]
		}
		out.PageInfo.HasPreviousPage = after != nil
	case wantLast > 0:
		if len(users) > wantLast {
			out.PageInfo.HasPreviousPage = true
			// Backward paging fetched (last+1) leading rows in reversed
			// SQL order; the repository already reversed them so the
			// extra row is at the head of the slice.
			users = users[len(users)-wantLast:]
		}
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
	if _, err := u.requireAdmin(ctx); err != nil {
		return nil, err
	}
	user, err := u.users.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, gqlerr.Cancelled(ctx, err)
		}
		return nil, gqlerr.Internal(ctx, eris.Wrap(err, "usecase: admin user get"))
	}
	return user, nil
}

// Update applies the patch to the user identified by id. Validation mirrors
// UserUsecase.UpdateUser: display_name must be 1-50 grapheme clusters when
// non-nil, bio must be at most 500 grapheme clusters when non-nil, and an
// explicit empty bio clears the column.
func (u *adminUserUsecase) Update(ctx context.Context, id string, input AdminUpdateUserInput) (*domain.User, error) {
	if _, err := u.requireAdmin(ctx); err != nil {
		return nil, err
	}

	patch := repository.UserUpdate{}
	if input.DisplayName != nil {
		trimmed := strings.TrimSpace(*input.DisplayName)
		if err := validateDisplayName(trimmed); err != nil {
			return nil, err
		}
		patch.DisplayName = &trimmed
	}
	if input.Bio != nil {
		if err := validateBio(input.Bio); err != nil {
			return nil, err
		}
		patch.Bio = input.Bio
	}

	user, err := u.users.Update(ctx, id, patch)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, gqlerr.BadUserInput("id", "user not found")
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, gqlerr.Cancelled(ctx, err)
		}
		return nil, gqlerr.Internal(ctx, eris.Wrap(err, "usecase: admin user update"))
	}
	return user, nil
}

// AssignRole grants roleID to userID. Idempotent at the repository layer:
// calling twice with the same ids is a no-op the second time.
//
// Error mapping: distinguishes user-missing from role-missing via the
// repository's ErrUserNotFound / ErrRoleNotFound sentinels so the
// BAD_USER_INPUT response carries the correct field for the frontend
// banner-by-field machinery. The generic ErrNotFound branch is kept as a
// fallback for any future repository implementation that surfaces only the
// legacy sentinel.
func (u *adminUserUsecase) AssignRole(ctx context.Context, userID, roleID string) (*domain.User, error) {
	if _, err := u.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := u.roles.AssignToUser(ctx, userID, roleID); err != nil {
		return nil, mapRoleAssignmentError(ctx, err, "usecase: admin user assign role")
	}
	return u.refetchUser(ctx, userID, "usecase: admin user assign role: refetch")
}

// RevokeRole removes roleID from userID. Self-demotion of the admin role is
// rejected with FORBIDDEN to prevent locking the system out of admin
// management. Idempotent at the repository layer otherwise.
func (u *adminUserUsecase) RevokeRole(ctx context.Context, userID, roleID string) (*domain.User, error) {
	callerID, err := u.requireAdmin(ctx)
	if err != nil {
		return nil, err
	}

	// Self-demotion guard: only blocks revoking the *admin* role from the
	// caller themselves. Look up the role first so the check is deterministic
	// even when the caller passes a stale roleID.
	if userID == callerID {
		roles, err := u.roles.FindByIDs(ctx, []string{roleID})
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, gqlerr.Cancelled(ctx, err)
			}
			return nil, gqlerr.Internal(ctx, eris.Wrap(err, "usecase: admin user revoke role: lookup role"))
		}
		if role, ok := roles[roleID]; ok && role.Name == adminRoleName {
			return nil, gqlerr.NewForbidden("cannot revoke own admin role")
		}
	}

	if err := u.roles.RevokeFromUser(ctx, userID, roleID); err != nil {
		return nil, mapRoleAssignmentError(ctx, err, "usecase: admin user revoke role")
	}
	return u.refetchUser(ctx, userID, "usecase: admin user revoke role: refetch")
}

// mapRoleAssignmentError translates the sentinel set returned by
// roleRepo.AssignToUser / RevokeFromUser into the typed gqlerr surface used by
// AssignRole and RevokeRole. Specific sentinels are matched before the legacy
// ErrNotFound fallback because both ErrUserNotFound and ErrRoleNotFound also
// satisfy errors.Is(_, ErrNotFound).
func mapRoleAssignmentError(ctx context.Context, err error, wrap string) error {
	switch {
	case errors.Is(err, repository.ErrUserNotFound):
		return gqlerr.BadUserInput("userId", "user not found")
	case errors.Is(err, repository.ErrRoleNotFound):
		return gqlerr.BadUserInput("roleId", "role not found")
	case errors.Is(err, repository.ErrNotFound):
		return gqlerr.BadUserInput("userId", "user or role not found")
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return gqlerr.Cancelled(ctx, err)
	default:
		return gqlerr.Internal(ctx, eris.Wrap(err, wrap))
	}
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
			return nil, gqlerr.Internal(ctx, eris.Wrapf(err, "%s: user disappeared", wrap))
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			return nil, gqlerr.Cancelled(ctx, err)
		default:
			return nil, gqlerr.Internal(ctx, eris.Wrap(err, wrap))
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
		return 0, 0, gqlerr.BadUserInput("first", "specify either first or last")
	}
	if first == nil && last == nil {
		return adminUserMaxPageSize, 0, nil
	}
	check := func(field string, v int) error {
		if v < 0 {
			return gqlerr.BadUserInput(field, fmt.Sprintf("%s must be >= 0", field))
		}
		if v > adminUserMaxPageSize {
			return gqlerr.BadUserInput(field, fmt.Sprintf("%s must be <= %d", field, adminUserMaxPageSize))
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
