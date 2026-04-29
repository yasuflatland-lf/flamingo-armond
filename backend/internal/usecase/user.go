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

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/gqlerr"
	"backend/internal/repository"
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
		return nil, gqlerr.Unauthenticated()
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
	return nil, gqlerr.Internal(ctx, err)
}

type UpdateUserInput struct {
	DisplayName string
	Bio         *string // nil = unchanged, "" = explicit clear
}

func (u *UserUsecase) UpdateUser(ctx context.Context, in UpdateUserInput) (*domain.User, error) {
	user := auth.UserFrom(ctx)
	if user == nil {
		return nil, gqlerr.Unauthenticated()
	}

	name := strings.TrimSpace(in.DisplayName)
	if err := validateDisplayName(name); err != nil {
		return nil, err
	}
	if err := validateBio(in.Bio); err != nil {
		return nil, err
	}

	appUser, err := u.repo.Update(ctx, user.Sub, repository.UserUpdate{
		DisplayName: &name,
		Bio:         in.Bio,
	})
	if err != nil {
		return nil, gqlerr.Internal(ctx, err)
	}
	return appUser, nil
}

func validateDisplayName(v string) error {
	n := uniseg.GraphemeClusterCount(v)
	if n < displayNameMin {
		return gqlerr.BadUserInput("displayName", "displayName is required")
	}
	if n > displayNameMax {
		return gqlerr.BadUserInput("displayName",
			fmt.Sprintf("displayName must be at most %d characters", displayNameMax))
	}
	return nil
}

func validateBio(v *string) error {
	if v == nil {
		return nil
	}
	n := uniseg.GraphemeClusterCount(*v)
	if n > bioMax {
		return gqlerr.BadUserInput("bio",
			fmt.Sprintf("bio must be at most %d characters", bioMax))
	}
	return nil
}
