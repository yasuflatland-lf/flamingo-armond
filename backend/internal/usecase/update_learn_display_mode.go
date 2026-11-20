package usecase

import (
	"context"

	"backend/internal/domain"
	"backend/internal/repository"
)

// UpdateLearnDisplayModeUsecase persists the authenticated caller's preferred
// learn display mode and returns the refreshed user row.
type UpdateLearnDisplayModeUsecase interface {
	Set(ctx context.Context, mode domain.LearnDisplayMode) (*domain.User, error)
}

// updateLearnDisplayModePrefsRepo is the narrow consumer interface for the
// user-preferences persistence step. Satisfied by
// repository.UserPreferenceRepository.
type updateLearnDisplayModePrefsRepo interface {
	UpsertLearnDisplayMode(ctx context.Context, userID, mode string) error
}

// updateLearnDisplayModeUsersRepo is the narrow consumer interface for the
// post-write user refetch step. Satisfied by repository.UserRepository.
type updateLearnDisplayModeUsersRepo interface {
	FindByID(ctx context.Context, id string) (*domain.User, error)
}

type updateLearnDisplayModeUsecase struct {
	prefs updateLearnDisplayModePrefsRepo
	users updateLearnDisplayModeUsersRepo
}

// NewUpdateLearnDisplayMode is the production constructor. Tests should prefer
// newUpdateLearnDisplayModeWithDeps to inject narrow stubs.
func NewUpdateLearnDisplayMode(
	prefs repository.UserPreferenceRepository,
	users repository.UserRepository,
) UpdateLearnDisplayModeUsecase {
	return &updateLearnDisplayModeUsecase{prefs: prefs, users: users}
}

// newUpdateLearnDisplayModeWithDeps accepts narrow interfaces for tests.
func newUpdateLearnDisplayModeWithDeps(
	prefs updateLearnDisplayModePrefsRepo,
	users updateLearnDisplayModeUsersRepo,
) UpdateLearnDisplayModeUsecase {
	return &updateLearnDisplayModeUsecase{prefs: prefs, users: users}
}

// Set persists mode as the caller's learn display preference and returns the
// refreshed user row. Authorization rules:
//
//   - Anonymous (no auth context) → UNAUTHENTICATED.
//   - Authenticated caller → updated preference + refreshed user row.
func (u *updateLearnDisplayModeUsecase) Set(ctx context.Context, mode domain.LearnDisplayMode) (*domain.User, error) {
	return setUserPreference(ctx, func(ctx context.Context, sub string) error {
		return u.prefs.UpsertLearnDisplayMode(ctx, sub, mode.String())
	}, u.users, "usecase: update learn display mode")
}
