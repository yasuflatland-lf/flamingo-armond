package usecase

import (
	"context"
	"log/slog"

	"github.com/rotisserie/eris"

	"backend/internal/auth"
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
	prefs  updateLearnDisplayModePrefsRepo
	users  updateLearnDisplayModeUsersRepo
	logger *slog.Logger
}

// NewUpdateLearnDisplayMode is the production constructor. Tests should prefer
// NewUpdateLearnDisplayModeWithDeps to inject narrow stubs.
func NewUpdateLearnDisplayMode(
	prefs repository.UserPreferenceRepository,
	users repository.UserRepository,
	logger *slog.Logger,
) UpdateLearnDisplayModeUsecase {
	if logger == nil {
		panic("usecase: update learn display mode: logger is required")
	}
	return &updateLearnDisplayModeUsecase{prefs: prefs, users: users, logger: logger}
}

// NewUpdateLearnDisplayModeWithDeps accepts narrow interfaces for tests.
func NewUpdateLearnDisplayModeWithDeps(
	prefs updateLearnDisplayModePrefsRepo,
	users updateLearnDisplayModeUsersRepo,
	logger *slog.Logger,
) UpdateLearnDisplayModeUsecase {
	if logger == nil {
		panic("usecase: update learn display mode: logger is required")
	}
	return &updateLearnDisplayModeUsecase{prefs: prefs, users: users, logger: logger}
}

// Set persists mode as the caller's learn display preference and returns the
// refreshed user row. Authorization rules:
//
//   - Anonymous (no auth context) → UNAUTHENTICATED.
//   - Authenticated caller → updated preference + refreshed user row.
func (u *updateLearnDisplayModeUsecase) Set(ctx context.Context, mode domain.LearnDisplayMode) (*domain.User, error) {
	caller := auth.UserFrom(ctx)
	if err := requireCallerSub(caller); err != nil {
		return nil, err
	}

	if err := u.prefs.UpsertLearnDisplayMode(ctx, caller.Sub, mode.String()); err != nil {
		if isContextDone(err) {
			return nil, err
		}
		return nil, eris.Wrap(err, "usecase: update learn display mode")
	}

	user, err := u.users.FindByID(ctx, caller.Sub)
	if err != nil {
		if isContextDone(err) {
			return nil, err
		}
		return nil, eris.Wrap(err, "usecase: update learn display mode: refetch own user row")
	}
	return user, nil
}
