// Package usecase wires authorization, repository calls, and domain rules
// behind GraphQL resolvers. Resolvers should not import repository directly.
package usecase

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/vektah/gqlparser/v2/gqlerror"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/repository"
)

const minDisplayName = 1
const maxDisplayName = 50

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

// Me returns the authenticated user's profile. Unauthenticated callers receive
// a gqlerror with extensions.code = "UNAUTHENTICATED" so clients can branch on it.
func (u *ProfileUsecase) Me(ctx context.Context) (*domain.Profile, error) {
	user := auth.UserFrom(ctx)
	if user == nil {
		return nil, unauthenticated()
	}
	p, err := u.repo.FindByID(ctx, user.Sub)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			slog.Warn("profile row missing for authenticated user; returning empty profile",
				"user_id", user.Sub,
				"hint", "expected handle_new_user trigger to provision row")
			return &domain.Profile{ID: user.Sub}, nil
		}
		return nil, err
	}
	return p, nil
}

type UpdateProfileInput struct {
	DisplayName string
	Bio         *string // nil = unchanged, "" = explicit clear
}

func (u *ProfileUsecase) UpdateProfile(ctx context.Context, in UpdateProfileInput) (*domain.Profile, error) {
	user := auth.UserFrom(ctx)
	if user == nil {
		return nil, unauthenticated()
	}

	name := strings.TrimSpace(in.DisplayName)
	if n := len([]rune(name)); n < minDisplayName || n > maxDisplayName {
		return nil, &gqlerror.Error{
			Message:    "displayName must be 1-50 characters",
			Extensions: map[string]any{"code": "BAD_USER_INPUT", "field": "displayName"},
		}
	}

	patch := repository.ProfileUpdate{
		DisplayName: &name,
		Bio:         in.Bio,
	}
	p, err := u.repo.Update(ctx, user.Sub, patch)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func unauthenticated() error {
	return &gqlerror.Error{
		Message:    "authentication required",
		Extensions: map[string]any{"code": "UNAUTHENTICATED"},
	}
}
