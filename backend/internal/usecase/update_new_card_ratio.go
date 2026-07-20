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
		return nil, translateNewCardRatioErr(err)
	}

	return setUserPreference(ctx, func(ctx context.Context, sub string) error {
		return u.prefs.UpsertNewCardRatio(ctx, sub, ratio.Numerator(), ratio.Denominator())
	}, u.users, "usecase: update new card ratio")
}

// translateNewCardRatioErr maps domain NewCardRatio sentinels into usecase-layer
// typed errors, attributing each rejection to the field the caller can fix: a
// share outside the open interval (0, denominator) faults the numerator; a
// non-positive or over-cap reduced denominator faults the denominator. The wire
// message stays generic so the internal bounds phrasing never leaks. Unexpected
// errors are wrapped with eris. Returns nil when err is nil.
func translateNewCardRatioErr(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, domain.ErrNewCardRatioShareOutOfRange):
		return ucerr.NewValidationError("numerator", "invalid new-card ratio")
	case errors.Is(err, domain.ErrNewCardRatioDenominatorNotPositive),
		errors.Is(err, domain.ErrNewCardRatioDenominatorTooLarge):
		return ucerr.NewValidationError("denominator", "invalid new-card ratio")
	default:
		return eris.Wrap(err, "usecase: update new card ratio: translate ratio error")
	}
}
