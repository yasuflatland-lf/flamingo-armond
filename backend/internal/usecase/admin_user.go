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

// errGuardAbort is a control-flow-only sentinel used to roll back the EditUser
// transaction when the self-demotion guard decides the edit must not proceed.
// It never escapes EditUser: the caller-visible result travels in the
// closure-captured guard outcome, not in this error. The guarantee holds
// because every `return errGuardAbort` site also sets guardHit = true, and the
// post-tx `if guardHit { return guard, nil }` check runs before the `if err !=
// nil` branch — so the sentinel is consumed and a nil error is returned to the
// caller.
var errGuardAbort = errors.New("usecase: admin user edit: guard abort")

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

// AdminEditUserInput captures the atomic admin edit shape. DisplayName and
// Bio are patch fields; RoleIDs is the final declarative role set.
type AdminEditUserInput struct {
	DisplayName     *string
	Bio             *string
	RoleIDs         []string
	ExpectedVersion int64
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
}

// adminRoleRepository is the subset of repository.RoleRepository used by the
// EditUser self-demotion guard. The lookup is tx-scoped and row-locking (FOR
// UPDATE) so role names are read without risk of a concurrent rename/delete
// between the guard check and the role-set write.
type adminRoleRepository interface {
	FindByIDsTx(ctx context.Context, tx *gorm.DB, ids []string) (map[string]*domain.Role, error)
}

// adminUserRoleRepository is the subset of repository.UserRoleRepository used
// by the AdminUser usecase (atomic membership replacement).
type adminUserRoleRepository interface {
	SetUserRolesTx(ctx context.Context, tx *gorm.DB, userID string, roleIDs []string) error
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
	if db != nil {
		uc.tx = func(ctx context.Context, fn func(tx *gorm.DB) error) error {
			return db.WithContext(ctx).Transaction(fn)
		}
	}
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
		out.PageInfo.StartCursor = &users[0].ID
		out.PageInfo.EndCursor = &users[len(users)-1].ID
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
// The self-demotion guard runs at the front of the transaction and reads role
// names with a FOR UPDATE lock (FindByIDsTx), so the role-name read is atomic
// with the role-set write. This closes the TOCTOU window where a concurrent
// admin could rename or delete the admin role between the guard read and the
// role-set replacement.
func (u *adminUserUsecase) EditUser(ctx context.Context, id string, input AdminEditUserInput) (AdminEditUserOutcome, error) {
	callerID, err := u.adminGate.Require(ctx, "usecase: admin user: check admin")
	if err != nil {
		return AdminEditUserOutcome{}, err
	}

	patch := repository.UserUpdate{}
	if input.DisplayName != nil {
		dn, err := domain.ParseDisplayName(*input.DisplayName)
		if err != nil {
			info, perr := liftValidationErr(translateDisplayNameErr(err))
			if perr != nil {
				return AdminEditUserOutcome{}, perr
			}
			return AdminEditUserOutcome{Validation: info}, nil
		}
		s := string(dn)
		patch.DisplayName = &s
	}
	if input.Bio != nil {
		bio, err := domain.ParseBio(input.Bio)
		if err != nil {
			info, perr := liftValidationErr(translateBioErr(err))
			if perr != nil {
				return AdminEditUserOutcome{}, perr
			}
			return AdminEditUserOutcome{Validation: info}, nil
		}
		patch.Bio = bio.Ptr()
	}

	roleIDs, validation := normalizeAdminEditRoleIDs(input.RoleIDs)
	if validation != nil {
		return AdminEditUserOutcome{Validation: validation}, nil
	}

	if u.tx == nil {
		return AdminEditUserOutcome{}, eris.New("usecase: admin user edit: tx runner not configured")
	}

	// guard carries the self-demotion outcome out of the tx closure. When
	// guardHit is set, the closure returned errGuardAbort to roll back the
	// transaction; the caller-visible result is guard, not the sentinel error.
	var guard AdminEditUserOutcome
	guardHit := false

	err = u.tx(ctx, func(tx *gorm.DB) error {
		if callerID == id {
			keepsAdmin := false
			if len(roleIDs) > 0 {
				roles, lerr := u.roles.FindByIDsTx(ctx, tx, roleIDs)
				if lerr != nil {
					if isContextDone(lerr) {
						return lerr
					}
					return eris.Wrap(lerr, "usecase: admin user edit: lookup roles")
				}
				// If any submitted roleId is unknown, surface that as a validation
				// failure rather than misclassifying the request as a self-demotion.
				// The downstream SetUserRolesTx would also reject the unknown id,
				// but only after passing the keepsAdmin check on the partial map.
				if len(roles) != len(roleIDs) {
					guard = AdminEditUserOutcome{Validation: NewInputValidationInfo("roleIds", "role not found")}
					guardHit = true
					return errGuardAbort
				}
				for _, roleID := range roleIDs {
					if role, ok := roles[roleID]; ok && role.Name == domain.AdminRoleName {
						keepsAdmin = true
						break
					}
				}
			}
			if !keepsAdmin {
				guard = AdminEditUserOutcome{CannotRevokeOwnAdmin: true}
				guardHit = true
				return errGuardAbort
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

	// Guard outcome takes precedence over the sentinel error it rode out on.
	if guardHit {
		return guard, nil
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

	user, err := u.refetchUser(ctx, id, "usecase: admin user edit: refetch")
	if err != nil {
		return AdminEditUserOutcome{}, err
	}
	return AdminEditUserOutcome{User: user}, nil
}

func mapAdminEditMutationError(err error) (*InputValidationInfo, error) {
	switch {
	case errors.Is(err, repository.ErrUserNotFound):
		return NewInputValidationInfo("id", "user not found"), nil
	case errors.Is(err, repository.ErrRoleNotFound):
		return NewInputValidationInfo("roleIds", "role not found"), nil
	case errors.Is(err, repository.ErrNotFound):
		return NewInputValidationInfo("id", "user or role not found"), nil
	case isContextDone(err):
		return nil, err
	default:
		return nil, eris.Wrap(err, "usecase: admin user edit: tx")
	}
}

func normalizeAdminEditRoleIDs(roleIDs []string) ([]string, *InputValidationInfo) {
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
