// Package usecase wires authorization, repository calls, and domain rules
// behind GraphQL resolvers. Resolvers should not import repository directly.
package usecase

import (
	"context"
	"errors"
	"log/slog"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/usecase/ucerr"
)

// UserRepository is the consumer-driven interface used by UserUsecase.
// FindByIDs is intentionally omitted; it is used only by the loader layer.
type UserRepository interface {
	FindByID(ctx context.Context, id string) (*domain.User, error)
	Update(ctx context.Context, id string, patch repository.UserUpdate) (*domain.User, error)
	// DeleteAuthUserTx deletes the caller's auth.users row inside the caller's
	// transaction, cascading to all associated data. See
	// repository.UserRepository.DeleteAuthUserTx for details.
	DeleteAuthUserTx(ctx context.Context, tx *gorm.DB, id string) error
	// AuthUserExists reports whether an auth.users row with the given id still
	// exists. Me uses it to tell a deleted account apart from a public.users row
	// the handle_new_user trigger has not written yet.
	AuthUserExists(ctx context.Context, id string) (bool, error)
}

type UserRolesRepository interface {
	ListByUser(ctx context.Context, userID string) ([]*domain.Role, error)
	// AcquireAdminRoleLockTx serializes admin-count-changing mutations; see
	// repository.UserRoleRepository for the race it closes.
	AcquireAdminRoleLockTx(ctx context.Context, tx *gorm.DB) error
	// CountAdminsTx returns the number of users holding the admin role, read
	// inside the caller's transaction under the lock above. Used by
	// DeleteMyAccount's last-admin guard.
	CountAdminsTx(ctx context.Context, tx *gorm.DB) (int64, error)
}

// UserUsecase is the authenticated user profile and role-query surface.
type UserUsecase interface {
	// Me returns the authenticated caller's profile. Returns
	// ucerr.ErrUnauthenticated when no caller is on the context, and also when
	// the caller's auth.users row is gone (the account was deleted while its
	// JWT was still valid) so the client signs the caller out.
	Me(ctx context.Context) (*domain.User, error)
	UpdateUser(ctx context.Context, in UpdateUserInput) (UpdateProfileOutcome, error)
	// DeleteMyAccount deletes the authenticated caller's own account and all
	// associated data. Returns ucerr.ErrUnauthenticated when no caller is on the
	// context, and a forbidden error when the caller is the last admin.
	DeleteMyAccount(ctx context.Context) error
}

type userUsecase struct {
	repo   UserRepository
	roles  UserRolesRepository
	auth   AdminChecker
	tx     txRunner
	logger *slog.Logger
}

// NewUserUsecase constructs the user profile usecase. db backs the transaction
// runner that scopes DeleteMyAccount's last-admin guard together with the
// delete; tests that inject repository fakes may pass nil (see runInTx).
func NewUserUsecase(db *gorm.DB, repo UserRepository, roles UserRolesRepository, authSvc AdminChecker, logger *slog.Logger) UserUsecase {
	if logger == nil {
		panic("usecase: user: logger is required")
	}
	return &userUsecase{repo: repo, roles: roles, auth: authSvc, tx: newTxRunner(db), logger: logger}
}

func (u *userUsecase) Me(ctx context.Context) (*domain.User, error) {
	user := auth.UserFrom(ctx)
	if err := requireCallerSub(user); err != nil {
		return nil, err
	}
	appUser, err := u.repo.FindByID(ctx, user.Sub)
	if err == nil {
		return appUser, nil
	}
	if errors.Is(err, repository.ErrNotFound) {
		// A missing public.users row has two causes that must not share an outcome:
		// the handle_new_user trigger has not provisioned it yet (a race on a brand
		// new sign-up), or the account was deleted while its JWT was still valid
		// (auth.users delete cascades to public.users). JWT verification is
		// stateless, so this read is the first place the deletion is observable.
		// Probe auth.users to tell them apart: absent means deleted, so return
		// ucerr.ErrUnauthenticated and let the client sign the caller out; present
		// means the provisioning race, which keeps the empty-user degrade.
		exists, existsErr := u.repo.AuthUserExists(ctx, user.Sub)
		if existsErr != nil {
			return nil, eris.Wrap(existsErr, "usecase: user: me: auth user exists")
		}
		if !exists {
			return nil, ucerr.ErrUnauthenticated
		}
		u.logger.WarnContext(ctx, "user row missing for authenticated user; returning empty user",
			"user_id", user.Sub)
		return &domain.User{ID: domain.UserID(user.Sub)}, nil
	}
	return nil, eris.Wrap(err, "usecase: user: me: find user by ID")
}

type UpdateUserInput struct {
	DisplayName string
	Bio         *string // nil = unchanged, "" or whitespace-only = explicit clear (trimmed); surrounding whitespace is stripped
}

// UpdateProfileOutcome is the result of UserUsecase.UpdateUser. Exactly one
// of User or Validation is non-nil on a nil-error return: a successful update
// carries the refreshed User; a displayName or bio that fails validation
// surfaces via Validation so the resolver maps it to the UpdateProfileResult
// union's InputValidationError variant.
type UpdateProfileOutcome struct {
	User       *domain.User
	Validation *InputValidationInfo
}

// UpdateUser validates and applies a profile patch for the authenticated
// caller. displayName is trimmed and checked against the 1–50 grapheme range;
// bio is optional (nil = unchanged, "" = explicit clear) and capped at 500
// graphemes. Validation failures are returned via the outcome's Validation
// field, not on the error channel.
func (u *userUsecase) UpdateUser(ctx context.Context, in UpdateUserInput) (UpdateProfileOutcome, error) {
	user := auth.UserFrom(ctx)
	if err := requireCallerSub(user); err != nil {
		return UpdateProfileOutcome{}, err
	}

	patch, info, err := buildUserProfilePatch(&in.DisplayName, in.Bio)
	if err != nil {
		return UpdateProfileOutcome{}, err
	}
	if info != nil {
		return UpdateProfileOutcome{Validation: info}, nil
	}

	appUser, err := u.repo.Update(ctx, user.Sub, patch)
	if err != nil {
		return UpdateProfileOutcome{}, eris.Wrap(err, "usecase: user: update: update user")
	}

	return UpdateProfileOutcome{User: appUser}, nil
}

// buildUserProfilePatch assembles a repository.UserUpdate from a profile-patch
// input, shared by userUsecase.UpdateUser (self-service) and
// adminUserUsecase.EditUser (admin). displayName is a *string so the one helper
// serves both surfaces: the self-service caller passes &in.DisplayName because
// its display name is required, while the admin caller passes the optional
// input.DisplayName pointer. When displayName is nil the field is left
// unchanged; when non-nil it is validated via domain.ParseDisplayName (which
// rejects empty), so the pointer absorbs the required/optional difference
// without changing behaviour. bio follows the trinary patch contract
// (nil = unchanged, "" = explicit clear) via domain.ParseBio.
//
// The return contract mirrors liftValidationErr's routing: a *ucerr.ValidationError
// surfaces in the second slot (*InputValidationInfo) with a nil error, so the
// caller can populate its outcome's Validation field; any other error surfaces
// in the third slot for the error channel. On success both are nil and the
// assembled patch is returned. Keeping the DTO primitive (repository.UserUpdate,
// not a VO) preserves the patch-DTO-primitive contract.
func buildUserProfilePatch(displayName *string, bio *string) (repository.UserUpdate, *InputValidationInfo, error) {
	patch := repository.UserUpdate{}
	if displayName != nil {
		dn, err := domain.ParseDisplayName(*displayName)
		if err != nil {
			info, perr := liftValidationErr(translateDisplayNameErr(err))
			if perr != nil {
				return repository.UserUpdate{}, nil, perr
			}
			return repository.UserUpdate{}, info, nil
		}
		name := string(dn)
		patch.DisplayName = &name
	}
	if bio != nil {
		b, err := domain.ParseBio(bio)
		if err != nil {
			info, perr := liftValidationErr(translateBioErr(err))
			if perr != nil {
				return repository.UserUpdate{}, nil, perr
			}
			return repository.UserUpdate{}, info, nil
		}
		patch.Bio = b.Ptr()
	}
	return patch, nil, nil
}

// DeleteMyAccount permanently deletes the authenticated caller's own account.
// The delete cascades through public.users and every child row via the schema's
// ON DELETE CASCADE foreign keys; no application-level multi-step delete is
// needed.
//
// Guard order:
//  1. Authentication: no caller on the context returns ucerr.ErrUnauthenticated.
//  2. Last-admin guard: if the caller holds the admin role and is the only admin,
//     return a forbidden error so the system is never left without an admin.
//     The admin-role advisory lock is taken at the front of the transaction,
//     before the caller's admin membership is read, so neither a concurrent
//     admin removal nor a concurrent promotion of the caller can race past the
//     count; the guard and the delete then commit under the same lock.
//
// A missing auth.users row at delete time is treated as idempotent success — the
// account is already gone from the system's perspective.
func (u *userUsecase) DeleteMyAccount(ctx context.Context) error {
	caller := auth.UserFrom(ctx)
	if err := requireCallerSub(caller); err != nil {
		return err
	}

	if u.auth == nil || u.roles == nil {
		return eris.New("usecase: user: delete my account: admin guard deps not configured")
	}
	return runInTx(ctx, u.tx, func(tx *gorm.DB) error {
		if lerr := acquireAdminRoleLock(ctx, tx, u.roles, "usecase: user: delete my account: count admins"); lerr != nil {
			return lerr
		}
		// Read the caller's admin membership only once the lock is held: a
		// caller promoted to admin between an unlocked read and the delete
		// would skip the guard and could empty the admin set.
		isAdmin, aerr := u.auth.IsAdmin(ctx, caller.Sub)
		if aerr != nil {
			if isContextDone(aerr) {
				return aerr
			}
			return eris.Wrap(aerr, "usecase: user: delete my account: check admin")
		}
		if gerr := guardNotLastAdmin(
			ctx,
			tx,
			isAdmin,
			u.roles,
			"usecase: user: delete my account: count admins",
			"cannot delete the last admin account; promote another admin first",
		); gerr != nil {
			return gerr
		}

		if derr := u.repo.DeleteAuthUserTx(ctx, tx, caller.Sub); derr != nil {
			if errors.Is(derr, repository.ErrNotFound) {
				return nil
			}
			if isContextDone(derr) {
				return derr
			}
			return eris.Wrap(derr, "usecase: user: delete my account")
		}
		return nil
	})
}
