package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"

	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/usecase/ucerr"
)

// AdminUserConnection is the usecase-level Relay-style page result for
// AdminUser.List. The resolver wraps it into model.UserConnection. It mirrors
// the other connection outputs (e.g. CardConnectionOutput): Users holds the raw
// page rows and StartCur/EndCur carry RAW user ids; the resolver applies
// cursor.Encode exactly once at the resolver→model boundary (see the "Cursor
// encoding happens exactly once" rule in .claude/rules/pagination.md).
type AdminUserConnection struct {
	Users      []*domain.User
	TotalCount int64
	HasNext    bool
	HasPrev    bool
	StartCur   string
	EndCur     string
}

// AdminEditUserInput captures the atomic admin edit shape. DisplayName and
// Bio are patch fields; RoleIDs is the final declarative role set.
type AdminEditUserInput struct {
	DisplayName     *string
	Bio             *string
	RoleIDs         []string
	ExpectedVersion int64
}

// AdminEditUserOutcome is the result of adminUserUsecase.EditUser. Exactly one
// of User, Validation, CannotRevokeOwnAdmin, or ConcurrentUpdate is active on
// a nil-error return.
type AdminEditUserOutcome struct {
	User                 *domain.User
	Validation           *InputValidationInfo
	CannotRevokeOwnAdmin bool
	ConcurrentUpdate     bool
}

// AdminUserUsecase is the admin-only user management surface.
type AdminUserUsecase interface {
	List(
		ctx context.Context,
		first, last *int,
		after, before, search *string,
	) (*AdminUserConnection, error)
	Get(ctx context.Context, id string) (*domain.User, error)
	EditUser(ctx context.Context, id string, input AdminEditUserInput) (AdminEditUserOutcome, error)
	// DeleteUser permanently deletes the user identified by id and all associated
	// data. Returns a forbidden error when the caller targets their own id or the
	// target is the last admin, and a validation error when the user is not found.
	DeleteUser(ctx context.Context, id string) error
}

// adminUserRepository is the subset of repository.UserRepository the
// AdminUser usecase consumes. Declaring a narrow interface keeps the
// usecase test scaffolding small.
type adminUserRepository interface {
	FindByID(ctx context.Context, id string) (*domain.User, error)
	UpdateTxVersioned(ctx context.Context, tx *gorm.DB, id string, patch repository.UserUpdate, expectedVersion int64) error
	ListPage(
		ctx context.Context,
		after, before *string,
		first, last int,
		search *string,
	) ([]*domain.User, int64, error)
	// DeleteAuthUserTx deletes the target's auth.users row inside the caller's
	// transaction, cascading to all associated data. See
	// repository.UserRepository.DeleteAuthUserTx for details.
	DeleteAuthUserTx(ctx context.Context, tx *gorm.DB, id string) error
}

// adminRoleRepository is the subset of repository.RoleRepository used by the
// EditUser self-demotion guard. The lookup is tx-scoped and row-locking (FOR
// UPDATE) so role names are read without risk of a concurrent rename/delete
// between the guard check and the role-set write.
type adminRoleRepository interface {
	FindByIDsTx(ctx context.Context, tx *gorm.DB, ids []string) (map[string]*domain.Role, error)
}

// adminUserRoleRepository is the subset of repository.UserRoleRepository used
// by the AdminUser usecase: atomic membership replacement (EditUser) and the
// last-admin / target-is-admin checks (EditUser's demote-other branch and
// DeleteUser). The lock + count pair is the AdminCounter port consumed by
// guardNotLastAdmin.
type adminUserRoleRepository interface {
	SetUserRolesTx(ctx context.Context, tx *gorm.DB, userID string, roleIDs []string) error
	// HasRoleTx reports whether the user holds the named role, read inside the
	// caller's transaction. Used to decide whether the last-admin guard applies
	// to the target; it must be read under AcquireAdminRoleLockTx so a
	// concurrent promotion cannot make the answer stale before the mutation.
	HasRoleTx(ctx context.Context, tx *gorm.DB, userID string, roleName domain.RoleName) (bool, error)
	// AcquireAdminRoleLockTx serializes admin-count-changing mutations; see
	// repository.UserRoleRepository for the race it closes.
	AcquireAdminRoleLockTx(ctx context.Context, tx *gorm.DB) error
	// CountAdminsTx returns the number of users holding the admin role, read
	// inside the caller's transaction under the lock above.
	CountAdminsTx(ctx context.Context, tx *gorm.DB) (int64, error)
}

// adminUserUsecase wires the admin gate, the user repository, the role
// repository, the user-role repository, and the transaction runner that scopes
// the atomic profile+roles write inside EditUser.
type adminUserUsecase struct {
	users     adminUserRepository
	roles     adminRoleRepository
	userRoles adminUserRoleRepository
	tx        txRunner
	adminGate *AdminGate
	logger    *slog.Logger
}

// NewAdminUser constructs an AdminUserUsecase. Pass production
// repository.UserRepository / repository.RoleRepository /
// repository.UserRoleRepository implementations; tests may pass narrower stubs
// that satisfy the package-private interfaces.
func NewAdminUser(
	db *gorm.DB,
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
	uc := &adminUserUsecase{users: users, roles: roles, userRoles: userRoles, adminGate: adminGate, logger: logger}
	uc.tx = newTxRunner(db)
	return uc
}

// NewAdminUserWithDeps is the test-time constructor that accepts the narrow
// interface types. Production code must use NewAdminUser.
func NewAdminUserWithDeps(
	users adminUserRepository,
	roles adminRoleRepository,
	userRoles adminUserRoleRepository,
	tx txRunner,
	adminGate *AdminGate,
	logger *slog.Logger,
) AdminUserUsecase {
	if adminGate == nil {
		panic("usecase: admin user: adminGate is required")
	}
	if logger == nil {
		panic("usecase: admin user: logger is required")
	}
	return &adminUserUsecase{users: users, roles: roles, userRoles: userRoles, tx: tx, adminGate: adminGate, logger: logger}
}

// List paginates the users table with Relay-style cursors. Forward paging
// uses (first, after); backward uses (last, before). The two are mutually
// exclusive. Default page size is maxPageSize (100); the same value is the
// absolute cap for either direction.
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

	wantFirst, wantLast, err := resolveRelayPage(first, last, after, before, resolveAdminPageSize)
	if err != nil {
		return nil, err
	}

	// Decode the opaque inbound cursors to raw user ids before the repository's
	// id-based hydration. A malformed v1 cursor is BAD_USER_INPUT; the repository
	// looks up the cursor row by raw id, so it must never receive the v1 envelope.
	afterCur, afterPresent, err := decodeCursorOrBadInput(after, "after")
	if err != nil {
		return nil, err
	}
	beforeCur, beforePresent, err := decodeCursorOrBadInput(before, "before")
	if err != nil {
		return nil, err
	}
	// The admin-users listing orders by the immutable created_at, so its cursors
	// stay on the v1 envelope and only the raw id is consumed here; a cursor
	// carrying ordering metadata cannot have come from this connection.
	if err := rejectOrderedCursor(afterCur, "after"); err != nil {
		return nil, err
	}
	if err := rejectOrderedCursor(beforeCur, "before"); err != nil {
		return nil, err
	}
	afterID, beforeID := afterCur.ID, beforeCur.ID
	var afterPtr, beforePtr *string
	if afterPresent {
		afterPtr = &afterID
	}
	if beforePresent {
		beforePtr = &beforeID
	}

	var total int64
	// afterPresent/beforePresent are the post-decode cursor presence that
	// assemblePage expects (a malformed cursor errored out above).
	users, hasNext, hasPrev, err := assemblePage(wantFirst, wantLast, afterPresent, beforePresent,
		func(repoFirst, repoLast int) ([]*domain.User, error) {
			rows, t, e := u.users.ListPage(ctx, afterPtr, beforePtr, repoFirst, repoLast, search)
			if e != nil {
				if errors.Is(e, repository.ErrCursorNotFound) {
					field := "after"
					if !afterPresent && beforePresent {
						field = "before"
					}
					return nil, ucerr.NewValidationError(field, "cursor not found")
				}
				if isContextDone(e) {
					return nil, e
				}
				return nil, eris.Wrap(e, "usecase: admin user: list")
			}
			total = t
			return rows, nil
		},
	)
	if err != nil {
		return nil, err
	}

	out := &AdminUserConnection{
		Users:      users,
		TotalCount: total,
		HasNext:    hasNext,
		HasPrev:    hasPrev,
	}
	// firstLastCursor returns "","" on an empty page; encodeCursor maps "" to a
	// nil cursor at the resolver, preserving the absent-cursor empty-page shape.
	out.StartCur, out.EndCur = firstLastCursor(users, func(u *domain.User) string { return string(u.ID) })
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

// EditUser atomically patches profile fields and replaces the user's role set.
// Validation failures surface as outcome data; when the caller is editing
// their own row and the submitted final role set drops the admin role,
// outcome.CannotRevokeOwnAdmin is set instead. The transaction covers both
// the profile update and role replacement; the returned user is refetched
// after the transaction commits so role loaders see the final state.
//
// The transaction runner is required for any patch that goes past validation.
// NewAdminUser leaves tx unwired when constructed with a nil db; in that case
// EditUser surfaces a wrapped 'tx runner not configured' error rather than
// panicking, since the wiring gap is recoverable per-request.
//
// The role-set guards run at the front of the transaction and read role names
// with a FOR UPDATE lock (FindByIDsTx), so the role-name read is atomic with
// the role-set write. This closes the TOCTOU window where a concurrent admin
// could rename or delete the admin role between the guard read and the role-set
// replacement. Two guards branch off the same "the submitted set drops admin"
// condition: editing your own row yields outcome.CannotRevokeOwnAdmin, while
// demoting somebody else takes the admin-role advisory lock, reads the target's
// admin membership under it, and only then runs guardNotLastAdmin — so neither
// two administrators demoting each other concurrently nor a target promoted
// after the membership read can leave the workspace with zero admins.
//
// When a guard blocks (self-demotion or an unknown submitted role) it returns
// nil from the tx closure after capturing the outcome in earlyOutcome — there
// is no write to roll back, so no control-flow sentinel is needed; the post-tx
// code returns the captured outcome before inspecting the transaction error.
// The last-admin guard is surfaced the same way through guardErr, which carries
// a forbidden error rather than an outcome field.
func (u *adminUserUsecase) EditUser(ctx context.Context, id string, input AdminEditUserInput) (AdminEditUserOutcome, error) {
	callerID, err := u.adminGate.Require(ctx, "usecase: admin user: check admin")
	if err != nil {
		return AdminEditUserOutcome{}, err
	}

	patch, info, err := buildUserProfilePatch(input.DisplayName, input.Bio)
	if err != nil {
		return AdminEditUserOutcome{}, err
	}
	if info != nil {
		return AdminEditUserOutcome{Validation: info}, nil
	}

	roleIDs, validation := normalizeAdminEditRoleIDs(input.RoleIDs)
	if validation != nil {
		return AdminEditUserOutcome{Validation: validation}, nil
	}

	if u.tx == nil {
		return AdminEditUserOutcome{}, eris.New("usecase: admin user edit: tx runner not configured")
	}

	// earlyOutcome carries a pre-write guard result (self-demotion blocked, or an
	// unknown submitted role) out of the tx closure. The guard runs before any
	// write, so blocking is an early `return nil` — there is nothing to roll back,
	// and no control-flow sentinel is needed.
	var earlyOutcome *AdminEditUserOutcome
	// guardErr carries the last-admin guard's forbidden error (or its
	// infrastructure failure) out of the tx closure. Like earlyOutcome it is
	// captured before any write, so the closure returns nil and the post-tx code
	// surfaces it without going through the mutation-error classifier.
	var guardErr error

	err = u.tx(ctx, func(tx *gorm.DB) error {
		// No write may be inserted ahead of these guards: a guard blocks via an
		// early `return nil`, which commits the transaction, so any prior write
		// would be persisted despite the block.
		keepsAdmin := false
		if len(roleIDs) > 0 {
			roles, lerr := u.roles.FindByIDsTx(ctx, tx, roleIDs)
			if lerr != nil {
				if isContextDone(lerr) {
					return lerr
				}
				return eris.Wrap(lerr, "usecase: admin user edit: lookup roles")
			}
			// An unknown submitted roleId is a validation failure, not a
			// self-demotion: the downstream SetUserRolesTx would also reject it,
			// but only after passing the keepsAdmin check on the partial map.
			// Other targets keep the downstream rejection so the relative
			// precedence of the id and roleIds validation errors is unchanged.
			if callerID == id && len(roles) != len(roleIDs) {
				earlyOutcome = &AdminEditUserOutcome{Validation: NewInputValidationInfo("roleIds", "role not found")}
				return nil
			}
			set := make(domain.RoleSet, 0, len(roles))
			for _, r := range roles {
				set = append(set, *r)
			}
			keepsAdmin = set.ContainsAdmin()
		}
		if !keepsAdmin {
			if callerID == id {
				earlyOutcome = &AdminEditUserOutcome{CannotRevokeOwnAdmin: true}
				return nil
			}
			// Demoting somebody else: the guard applies only when the target
			// currently holds the role the submitted set drops. The advisory
			// lock is taken before that membership read, not inside the guard,
			// so a target promoted concurrently cannot be read as a non-admin
			// and skip the guard altogether.
			if lerr := acquireAdminRoleLock(ctx, tx, u.userRoles, "usecase: admin user edit: count admins"); lerr != nil {
				guardErr = lerr
				return nil
			}
			isAdmin, herr := u.userRoles.HasRoleTx(ctx, tx, id, domain.AdminRoleName)
			if herr != nil {
				if isContextDone(herr) {
					return herr
				}
				return eris.Wrap(herr, "usecase: admin user edit: check admin role")
			}
			if gerr := guardNotLastAdmin(
				ctx,
				tx,
				isAdmin,
				u.userRoles,
				"usecase: admin user edit: count admins",
				"cannot remove the admin role from the last admin account",
			); gerr != nil {
				guardErr = gerr
				return nil
			}
		}

		if uerr := u.users.UpdateTxVersioned(ctx, tx, id, patch, input.ExpectedVersion); uerr != nil {
			if isContextDone(uerr) {
				return uerr
			}
			return eris.Wrap(uerr, "usecase: admin user edit: update profile")
		}
		if serr := u.userRoles.SetUserRolesTx(ctx, tx, id, roleIDs); serr != nil {
			if isContextDone(serr) {
				return serr
			}
			return eris.Wrap(serr, "usecase: admin user edit: replace roles")
		}
		return nil
	})

	if earlyOutcome != nil {
		return *earlyOutcome, nil
	}
	if guardErr != nil {
		return AdminEditUserOutcome{}, guardErr
	}
	if err != nil {
		if isContextDone(err) {
			return AdminEditUserOutcome{}, err
		}
		if errors.Is(err, repository.ErrConcurrentUpdate) {
			return AdminEditUserOutcome{ConcurrentUpdate: true}, nil
		}
		info, perr := mapAdminEditMutationError(err)
		if perr != nil {
			return AdminEditUserOutcome{}, perr
		}
		return AdminEditUserOutcome{Validation: info}, nil
	}

	user, err := refetchUser(ctx, u.users, id, "usecase: admin user edit: refetch")
	if err != nil {
		return AdminEditUserOutcome{}, err
	}
	return AdminEditUserOutcome{User: user}, nil
}

// DeleteUser permanently deletes the user identified by id and all associated
// data. The delete cascades through public.users and every child row via the
// schema's ON DELETE CASCADE foreign keys; no application-level multi-step
// delete is needed.
//
// Guard order:
//  1. Admin gate: non-admin / unauthenticated callers are rejected.
//  2. Self-deletion block: an admin cannot delete their own account from the
//     admin surface (they must use DeleteMyAccount), preventing an accidental
//     lockout.
//  3. Last-admin guard: when the target holds the admin role and is the only
//     admin, the deletion is refused so the system is never left without an
//     admin. The membership read, the count and the delete share one
//     transaction that holds the admin-role advisory lock from before the
//     membership read, so neither a concurrent admin removal nor a concurrent
//     promotion of the target can slip between the checks and the delete.
//
// A missing target maps to a validation error on "id" (BAD_USER_INPUT).
func (u *adminUserUsecase) DeleteUser(ctx context.Context, id string) error {
	callerID, err := u.adminGate.Require(ctx, "usecase: admin user: check admin")
	if err != nil {
		return err
	}
	if callerID == id {
		return ucerr.NewForbiddenError("cannot delete your own account from the admin panel; use deleteMyAccount")
	}

	return runInTx(ctx, u.tx, func(tx *gorm.DB) error {
		// The admin-role lock is taken before the membership read so a target
		// promoted concurrently cannot be read as a non-admin and bypass the
		// count entirely; only then is the global admin count consulted, and
		// only when the target is itself an admin.
		if lerr := acquireAdminRoleLock(ctx, tx, u.userRoles, "usecase: admin user: delete: count admins"); lerr != nil {
			return lerr
		}
		isAdmin, herr := u.userRoles.HasRoleTx(ctx, tx, id, domain.AdminRoleName)
		if herr != nil {
			if isContextDone(herr) {
				return herr
			}
			return eris.Wrap(herr, "usecase: admin user: delete: check admin role")
		}
		if gerr := guardNotLastAdmin(
			ctx,
			tx,
			isAdmin,
			u.userRoles,
			"usecase: admin user: delete: count admins",
			"cannot delete the last admin account",
		); gerr != nil {
			return gerr
		}

		if derr := u.users.DeleteAuthUserTx(ctx, tx, id); derr != nil {
			if errors.Is(derr, repository.ErrNotFound) {
				return ucerr.NewValidationError("id", "user not found")
			}
			if isContextDone(derr) {
				return derr
			}
			return eris.Wrap(derr, "usecase: admin user: delete")
		}
		return nil
	})
}

func mapAdminEditMutationError(err error) (*InputValidationInfo, error) {
	return classifyRepoErr(err, "usecase: admin user edit: tx", []SentinelMapping{
		{repository.ErrUserNotFound, "id", "user not found"},
		{repository.ErrRoleNotFound, "roleIds", "role not found"},
		{repository.ErrNotFound, "id", "user or role not found"},
	})
}

// maxAdminEditRoleIDs caps the number of role ids accepted by a single
// adminEditUser call. The cap is a defensive bound mirroring the bulk-path
// convention (maxBulkDelete = 100): an unbounded slice would allocate O(n) maps
// and issue a WHERE id IN (… n …) query inside a FOR UPDATE transaction. The
// real role universe is tiny, so 100 is generous headroom.
const maxAdminEditRoleIDs = 100

func normalizeAdminEditRoleIDs(roleIDs []string) ([]string, *InputValidationInfo) {
	if len(roleIDs) > maxAdminEditRoleIDs {
		return nil, NewInputValidationInfo("roleIds", fmt.Sprintf("at most %d ids per call", maxAdminEditRoleIDs))
	}
	out := make([]string, 0, len(roleIDs))
	seen := make(map[string]bool, len(roleIDs))
	for _, roleID := range roleIDs {
		if roleID == "" {
			return nil, NewInputValidationInfo("roleIds", "role ID is required")
		}
		if seen[roleID] {
			return nil, NewInputValidationInfo("roleIds", "role IDs must be unique")
		}
		seen[roleID] = true
		out = append(out, roleID)
	}
	return out, nil
}
