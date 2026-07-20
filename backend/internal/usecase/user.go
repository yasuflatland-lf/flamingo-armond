// Package usecase wires authorization, repository calls, and domain rules
// behind GraphQL resolvers. Resolvers should not import repository directly.
package usecase

import (
	"context"
	"errors"
	"log/slog"

	"github.com/rotisserie/eris"

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
	// DeleteAuthUser deletes the caller's auth.users row, cascading to all
	// associated data. See repository.UserRepository.DeleteAuthUser for details.
	DeleteAuthUser(ctx context.Context, id string) error
	// AuthUserExists reports whether an auth.users row with the given id still
	// exists. Me uses it to tell a deleted account apart from a public.users row
	// the handle_new_user trigger has not written yet.
	AuthUserExists(ctx context.Context, id string) (bool, error)
}

type UserRolesRepository interface {
	ListByUser(ctx context.Context, userID string) ([]*domain.Role, error)
	// CountAdmins returns the number of users holding the admin role. Used by
	// DeleteMyAccount's last-admin guard.
	CountAdmins(ctx context.Context) (int64, error)
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
	logger *slog.Logger
}

// NewUserUsecase constructs the user profile usecase.
func NewUserUsecase(repo UserRepository, roles UserRolesRepository, authSvc AdminChecker, logger *slog.Logger) UserUsecase {
	if logger == nil {
		panic("usecase: user: logger is required")
	}
	return &userUsecase{repo: repo, roles: roles, auth: authSvc, logger: logger}
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
//     This is best-effort (no row lock): a concurrent admin deletion could race
//     past the count. At this app's scale the TOCTOU window is acceptable; a
//     Postgres advisory lock is the upgrade path if it ever matters.
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
	isAdmin, err := u.auth.IsAdmin(ctx, caller.Sub)
	if err != nil {
		if isContextDone(err) {
			return err
		}
		return eris.Wrap(err, "usecase: user: delete my account: check admin")
	}
	if err := guardNotLastAdmin(
		ctx,
		isAdmin,
		u.roles,
		"usecase: user: delete my account: count admins",
		"cannot delete the last admin account; promote another admin first",
	); err != nil {
		return err
	}

	if err := u.repo.DeleteAuthUser(ctx, caller.Sub); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil
		}
		if isContextDone(err) {
			return err
		}
		return eris.Wrap(err, "usecase: user: delete my account")
	}
	return nil
}
