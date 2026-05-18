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
}

type UserRolesRepository interface {
	ListByUser(ctx context.Context, userID string) ([]*domain.Role, error)
}

type UserUsecase struct {
	repo   UserRepository
	roles  UserRolesRepository
	auth   AdminChecker
	logger *slog.Logger
}

func NewUserUsecase(repo UserRepository, roles UserRolesRepository, authSvc AdminChecker, logger *slog.Logger) *UserUsecase {
	if logger == nil {
		panic("usecase: user: logger is required")
	}
	return &UserUsecase{repo: repo, roles: roles, auth: authSvc, logger: logger}
}

func (u *UserUsecase) Me(ctx context.Context) (*domain.User, error) {
	user := auth.UserFrom(ctx)
	if user == nil {
		return nil, ucerr.ErrUnauthenticated
	}
	appUser, err := u.repo.FindByID(ctx, user.Sub)
	if err == nil {
		return appUser, nil
	}
	if errors.Is(err, repository.ErrNotFound) {
		// handle_new_user trigger should have provisioned the row; degrade gracefully.
		u.logger.WarnContext(ctx, "user row missing for authenticated user; returning empty user",
			"user_id", user.Sub)
		return &domain.User{ID: user.Sub}, nil
	}
	return nil, eris.Wrap(err, "usecase: Me: find user by ID")
}

func (u *UserUsecase) RolesFor(ctx context.Context, targetID string) ([]*domain.Role, error) {
	caller := auth.UserFrom(ctx)
	if caller == nil || caller.Sub == "" {
		return nil, ucerr.ErrUnauthenticated
	}
	if caller.Sub != targetID {
		if u.auth == nil {
			return nil, eris.New("usecase: user roles: admin checker not configured")
		}
		ok, err := u.auth.IsAdmin(ctx, caller.Sub)
		if err != nil {
			if isContextDone(err) {
				return nil, err
			}
			return nil, eris.Wrap(err, "usecase: user roles: check admin")
		}
		if !ok {
			return nil, ucerr.NewForbiddenError("admin only")
		}
	}
	if u.roles == nil {
		return nil, eris.New("usecase: user roles: roles repository not configured")
	}
	roles, err := u.roles.ListByUser(ctx, targetID)
	if err != nil {
		if isContextDone(err) {
			return nil, err
		}
		return nil, eris.Wrap(err, "usecase: user roles: list by user")
	}
	return roles, nil
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
func (u *UserUsecase) UpdateUser(ctx context.Context, in UpdateUserInput) (UpdateProfileOutcome, error) {
	user := auth.UserFrom(ctx)
	if user == nil {
		return UpdateProfileOutcome{}, ucerr.ErrUnauthenticated
	}

	dn, err := domain.ParseDisplayName(in.DisplayName)
	if err != nil {
		info, perr := liftValidationErr(translateDisplayNameErr(err))
		if perr != nil {
			return UpdateProfileOutcome{}, perr
		}
		return UpdateProfileOutcome{Validation: info}, nil
	}

	name := string(dn)
	patch := repository.UserUpdate{
		DisplayName: &name,
	}
	if in.Bio != nil {
		bio, err := domain.ParseBio(in.Bio)
		if err != nil {
			info, perr := liftValidationErr(translateBioErr(err))
			if perr != nil {
				return UpdateProfileOutcome{}, perr
			}
			return UpdateProfileOutcome{Validation: info}, nil
		}
		patch.Bio = bio.Ptr()
	}
	appUser, err := u.repo.Update(ctx, user.Sub, patch)
	if err != nil {
		return UpdateProfileOutcome{}, eris.Wrap(err, "usecase: UpdateUser: update user")
	}
	return UpdateProfileOutcome{User: appUser}, nil
}
