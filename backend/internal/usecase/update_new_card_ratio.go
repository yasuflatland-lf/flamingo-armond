package usecase

import (
	"context"
	"log/slog"

	"github.com/rotisserie/eris"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/repository"
)

// UpdateNewCardRatioUsecase persists the authenticated caller's preferred
// new-card ratio and returns the refreshed user row.
type UpdateNewCardRatioUsecase interface {
	Set(ctx context.Context, ratio domain.NewCardRatio) (*domain.User, error)
}

// updateNewCardRatioPrefsRepo is the narrow consumer interface for the
// user-preferences persistence step. Satisfied by
// repository.UserPreferenceRepository.
type updateNewCardRatioPrefsRepo interface {
	UpdateNewCardRatio(ctx context.Context, userID string, num, den int) error
}

// updateNewCardRatioUsersRepo is the narrow consumer interface for the
// post-write user refetch step. Satisfied by repository.UserRepository.
type updateNewCardRatioUsersRepo interface {
	FindByID(ctx context.Context, id string) (*domain.User, error)
}

type updateNewCardRatioUsecase struct {
	prefs  updateNewCardRatioPrefsRepo
	users  updateNewCardRatioUsersRepo
	logger *slog.Logger
}

// NewUpdateNewCardRatio is the production constructor. Tests should prefer
// NewUpdateNewCardRatioWithDeps to inject narrow stubs.
func NewUpdateNewCardRatio(
	prefs repository.UserPreferenceRepository,
	users repository.UserRepository,
	logger *slog.Logger,
) UpdateNewCardRatioUsecase {
	if logger == nil {
		panic("usecase: update new card ratio: logger is required")
	}
	return &updateNewCardRatioUsecase{prefs: prefs, users: users, logger: logger}
}

// NewUpdateNewCardRatioWithDeps accepts narrow interfaces for tests.
func NewUpdateNewCardRatioWithDeps(
	prefs updateNewCardRatioPrefsRepo,
	users updateNewCardRatioUsersRepo,
	logger *slog.Logger,
) UpdateNewCardRatioUsecase {
	if logger == nil {
		panic("usecase: update new card ratio: logger is required")
	}
	return &updateNewCardRatioUsecase{prefs: prefs, users: users, logger: logger}
}

// Set persists ratio as the caller's new-card preference and returns the
// refreshed user row. Authorization rules:
//
//   - Anonymous (no auth context) → UNAUTHENTICATED.
//   - Authenticated caller → updated preference + refreshed user row.
//
// ratio is an already-reduced, already-validated domain.NewCardRatio; the
// caller (the resolver) constructs it via domain.ParseNewCardRatio, so a bad
// wire value never reaches this method.
func (u *updateNewCardRatioUsecase) Set(ctx context.Context, ratio domain.NewCardRatio) (*domain.User, error) {
	caller := auth.UserFrom(ctx)
	if err := requireCallerSub(caller); err != nil {
		return nil, err
	}

	if err := u.prefs.UpdateNewCardRatio(ctx, caller.Sub, ratio.Numerator(), ratio.Denominator()); err != nil {
		if isContextDone(err) {
			return nil, err
		}
		return nil, eris.Wrap(err, "usecase: update new card ratio")
	}

	user, err := u.users.FindByID(ctx, caller.Sub)
	if err != nil {
		if isContextDone(err) {
			return nil, err
		}
		return nil, eris.Wrap(err, "usecase: update new card ratio: refetch own user row")
	}
	return user, nil
}
