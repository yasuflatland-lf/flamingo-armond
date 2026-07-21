package usecase

import (
	"context"
	"errors"
	"log/slog"

	"github.com/rotisserie/eris"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/repository"
)

// SetLastViewedCardgroupOutcome is the result of LastViewedCardgroupUsecase.Set.
// Exactly one of User or Validation is non-nil on a nil-error return: a
// successful upsert carries the refreshed User; a cardgroup that is not found
// or not owned by the caller surfaces via Validation so the resolver maps it
// to the SetLastViewedCardgroupResult union's InputValidationError variant.
type SetLastViewedCardgroupOutcome struct {
	User       *domain.User
	Validation *InputValidationInfo
}

// LastViewedCardgroupUsecase records the cardgroup the authenticated caller
// most recently viewed on /learn. The mutation is idempotent and ownership-
// checked at the repository layer (single SQL statement, no TOCTOU window the
// usecase needs to widen).
type LastViewedCardgroupUsecase interface {
	Set(ctx context.Context, cardgroupID string) (SetLastViewedCardgroupOutcome, error)
}

type lastViewedCardgroupRepo interface {
	UpsertLastViewedCardgroup(ctx context.Context, userID, cardgroupID string) error
}

type lastViewedCardgroupUsersRepo interface {
	FindByID(ctx context.Context, id string) (*domain.User, error)
}

type lastViewedCardgroupUsecase struct {
	prefs  lastViewedCardgroupRepo
	users  lastViewedCardgroupUsersRepo
	logger *slog.Logger
}

// NewLastViewedCardgroup is the production constructor. Tests should prefer
// newLastViewedCardgroupWithDeps to inject narrow stubs.
func NewLastViewedCardgroup(
	prefs repository.UserPreferenceRepository,
	users repository.UserRepository,
	logger *slog.Logger,
) LastViewedCardgroupUsecase {
	if logger == nil {
		panic("usecase: last viewed cardgroup: logger is required")
	}
	return &lastViewedCardgroupUsecase{prefs: prefs, users: users, logger: logger}
}

// newLastViewedCardgroupWithDeps accepts narrow interfaces for tests.
func newLastViewedCardgroupWithDeps(
	prefs lastViewedCardgroupRepo,
	users lastViewedCardgroupUsersRepo,
	logger *slog.Logger,
) LastViewedCardgroupUsecase {
	if logger == nil {
		panic("usecase: last viewed cardgroup: logger is required")
	}
	return &lastViewedCardgroupUsecase{prefs: prefs, users: users, logger: logger}
}

// Set records cardgroupID as the caller's most recently viewed cardgroup and
// returns the refreshed user row via SetLastViewedCardgroupOutcome. Authorization rules:
//
//   - Anonymous (no auth context) → UNAUTHENTICATED.
//   - Cardgroup not owned OR cardgroup does not exist → Validation variant
//     (cardgroupId). The same variant is returned for both cases so existence
//     of other users' cardgroups is not leaked through the error shape.
//   - Owned, existing cardgroup → User variant.
//
// Concurrency: a cardgroup deleted between the EXISTS check and the UPDATE
// surfaces as a 23503 FK violation, which the repository classifies into
// ErrCardgroupNotFound so the caller receives the Validation variant rather
// than an error.
func (u *lastViewedCardgroupUsecase) Set(ctx context.Context, cardgroupID string) (SetLastViewedCardgroupOutcome, error) {
	caller := auth.UserFrom(ctx)
	if err := requireCallerSub(caller); err != nil {
		return SetLastViewedCardgroupOutcome{}, err
	}

	if err := u.prefs.UpsertLastViewedCardgroup(ctx, caller.Sub, cardgroupID); err != nil {
		switch {
		case errors.Is(err, repository.ErrCardgroupNotFound):
			return SetLastViewedCardgroupOutcome{
				Validation: NewInputValidationInfo("cardgroupId", "cardgroup not found or not owned"),
			}, nil
		case isContextDone(err):
			return SetLastViewedCardgroupOutcome{}, err
		default:
			return SetLastViewedCardgroupOutcome{}, eris.Wrap(err, "usecase: set last viewed cardgroup")
		}
	}

	// ErrNotFound from the refetch means the user row vanished between the
	// UPDATE and the read — only possible if the auth.users row was deleted
	// concurrently. refetchUser treats it as INTERNAL (wrapping the sentinel as
	// "... : user disappeared") like any other refetch failure, and passes
	// context cancellation through unchanged.
	user, err := refetchUser(ctx, u.users, caller.Sub, "usecase: last viewed cardgroup: refetch own user row")
	if err != nil {
		return SetLastViewedCardgroupOutcome{}, err
	}
	return SetLastViewedCardgroupOutcome{User: user}, nil
}
