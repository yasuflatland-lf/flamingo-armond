package usecase

import (
	"context"
	"log/slog"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/usecase/ucerr"
)

// UpdateNewCardRatioUsecase persists the authenticated caller's preferred
// new-card ratio and returns the refreshed user row.
type UpdateNewCardRatioUsecase interface {
	Set(ctx context.Context, numerator, denominator int) (*domain.User, error)
}

// updateNewCardRatioPrefsRepo is the narrow consumer interface for the
// user-preferences persistence step. Satisfied by
// repository.UserPreferenceRepository.
type updateNewCardRatioPrefsRepo interface {
	UpsertNewCardRatio(ctx context.Context, userID string, num, den int) error
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

// Set validates numerator/denominator, persists the reduced ratio as the
// caller's new-card preference, and returns the refreshed user row.
// Authorization and validation rules:
//
//   - Anonymous (no auth context) → UNAUTHENTICATED. The auth check runs before
//     validation so an invalid ratio never reveals the bounds to an
//     unauthenticated caller.
//   - A ratio outside 1 <= numerator < denominator <= 100 (after reduction) → a
//     field-scoped ValidationError the resolver maps to BAD_USER_INPUT.
//   - Authenticated caller with a valid ratio → updated preference + refreshed
//     user row.
//
// The usecase owns domain.ParseNewCardRatio (previously the resolver's job) so
// value-object validation and field attribution live in the application layer,
// like every sibling mutation.
func (u *updateNewCardRatioUsecase) Set(ctx context.Context, numerator, denominator int) (*domain.User, error) {
	// Auth runs before validation so an invalid ratio never short-circuits the
	// unauthenticated path (an anonymous caller gets UNAUTHENTICATED, never a
	// hint that the ratio was malformed). setUserPreference re-checks auth, but
	// this guard is what fixes the auth->validate ordering the shared core,
	// which authenticates internally, cannot express on its own.
	if err := requireCallerSub(auth.UserFrom(ctx)); err != nil {
		return nil, err
	}

	ratio, err := domain.ParseNewCardRatio(numerator, denominator)
	if err != nil {
		// Attribute the fault the way domain.ParseNewCardRatio does on the
		// reduced fraction: a non-positive denominator, or a reduced denominator
		// above NewCardRatioDenMax, is a denominator problem; a new-card share
		// outside the open interval (0, denominator) is a numerator problem. The
		// share bound is ratio-invariant (num/den < 1 iff rnum/rden < 1), so this
		// verdict matches the reduced fraction the VO actually checks. The wire
		// message stays generic to avoid leaking internal bounds phrasing.
		field := "denominator"
		if denominator > 0 && (numerator <= 0 || numerator >= denominator) {
			field = "numerator"
		}
		return nil, ucerr.NewValidationError(field, "invalid new-card ratio")
	}

	return setUserPreference(ctx, func(ctx context.Context, sub string) error {
		return u.prefs.UpsertNewCardRatio(ctx, sub, ratio.Numerator(), ratio.Denominator())
	}, u.users, "usecase: update new card ratio")
}
