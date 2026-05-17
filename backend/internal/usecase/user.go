// Package usecase wires authorization, repository calls, and domain rules
// behind GraphQL resolvers. Resolvers should not import repository directly.
package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/rivo/uniseg"
	"github.com/rotisserie/eris"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/usecase/ucerr"
)

const (
	displayNameMin = 1
	displayNameMax = 50
	bioMax         = 500
)

// UserRepository is the consumer-driven interface used by UserUsecase.
// FindByIDs is intentionally omitted; it is used only by the loader layer.
type UserRepository interface {
	FindByID(ctx context.Context, id string) (*domain.User, error)
	Update(ctx context.Context, id string, patch repository.UserUpdate) (*domain.User, error)
}

type UserUsecase struct {
	repo UserRepository
}

func NewUserUsecase(repo UserRepository) *UserUsecase {
	return &UserUsecase{repo: repo}
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
		slog.Warn("user row missing for authenticated user; returning empty user",
			"user_id", user.Sub)
		return &domain.User{ID: user.Sub}, nil
	}
	return nil, eris.Wrap(err, "usecase: Me: find user by ID")
}

type UpdateUserInput struct {
	DisplayName string
	Bio         *string // nil = unchanged, "" = explicit clear
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

	name := strings.TrimSpace(in.DisplayName)
	info, err := liftValidationErr(validateDisplayName(name))
	if err != nil {
		return UpdateProfileOutcome{}, err
	}
	if info != nil {
		return UpdateProfileOutcome{Validation: info}, nil
	}

	info, err = liftValidationErr(validateBio(in.Bio))
	if err != nil {
		return UpdateProfileOutcome{}, err
	}
	if info != nil {
		return UpdateProfileOutcome{Validation: info}, nil
	}

	appUser, err := u.repo.Update(ctx, user.Sub, repository.UserUpdate{
		DisplayName: &name,
		Bio:         in.Bio,
	})
	if err != nil {
		return UpdateProfileOutcome{}, eris.Wrap(err, "usecase: UpdateUser: update user")
	}
	return UpdateProfileOutcome{User: appUser}, nil
}

func validateDisplayName(v string) error {
	n := uniseg.GraphemeClusterCount(v)
	if n < displayNameMin {
		return ucerr.NewValidationError("displayName", "displayName is required")
	}
	if n > displayNameMax {
		return ucerr.NewValidationError("displayName", fmt.Sprintf("displayName must be at most %d characters", displayNameMax))
	}
	return nil
}

func validateBio(v *string) error {
	if v == nil {
		return nil
	}
	n := uniseg.GraphemeClusterCount(*v)
	if n > bioMax {
		return ucerr.NewValidationError("bio", fmt.Sprintf("bio must be at most %d characters", bioMax))
	}
	return nil
}
