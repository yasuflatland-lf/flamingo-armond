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

// ProfileRepository is the consumer-driven interface used by ProfileUsecase.
// FindByIDs is intentionally omitted; it is used only by the loader layer.
type ProfileRepository interface {
	FindByID(ctx context.Context, id string) (*domain.Profile, error)
	Update(ctx context.Context, id string, patch repository.ProfileUpdate) (*domain.Profile, error)
}

type ProfileUsecase struct {
	repo ProfileRepository
}

func NewProfileUsecase(repo ProfileRepository) *ProfileUsecase {
	return &ProfileUsecase{repo: repo}
}

func (u *ProfileUsecase) Me(ctx context.Context) (*domain.Profile, error) {
	user := auth.UserFrom(ctx)
	if user == nil {
		return nil, gqlerr.Unauthenticated()
	}
	p, err := u.repo.FindByID(ctx, user.Sub)
	if err == nil {
		return p, nil
	}
	if errors.Is(err, repository.ErrNotFound) {
		// handle_new_user trigger should have provisioned the row; degrade gracefully.
		slog.Warn("profile row missing for authenticated user; returning empty profile",
			"user_id", user.Sub)
		return &domain.Profile{ID: user.Sub}, nil
	}
	return nil, gqlerr.Internal(ctx, err)
}

type UpdateProfileInput struct {
	DisplayName string
	Bio         *string // nil = unchanged, "" = explicit clear
}

func (u *ProfileUsecase) UpdateProfile(ctx context.Context, in UpdateProfileInput) (*domain.Profile, error) {
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

	p, err := u.repo.Update(ctx, user.Sub, repository.ProfileUpdate{
		DisplayName: &name,
		Bio:         in.Bio,
	})
	if err != nil {
		return nil, gqlerr.Internal(ctx, err)
	}
	return p, nil
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
