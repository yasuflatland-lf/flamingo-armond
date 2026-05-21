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

type userPreferenceRefetchRepo interface {
	FindByID(ctx context.Context, id string) (*domain.User, error)
}

type lastViewedCardgroupUsecase struct {
	prefs  lastViewedCardgroupRepo
	users  userPreferenceRefetchRepo
	logger *slog.Logger
}

// NewLastViewedCardgroup is the production constructor. Tests should prefer
// NewLastViewedCardgroupWithDeps to inject narrow stubs.
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

// NewLastViewedCardgroupWithDeps accepts narrow interfaces for tests.
func NewLastViewedCardgroupWithDeps(
	prefs lastViewedCardgroupRepo,
	users userPreferenceRefetchRepo,
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

	user, err := u.users.FindByID(ctx, caller.Sub)
	if err != nil {
		// ErrNotFound here means the user row vanished between the UPDATE and
		// the refetch — only possible if the auth.users row was deleted
		// concurrently. Treated as INTERNAL like any other refetch failure,
		// with the wrapped sentinel preserved for log correlation.
		if isContextDone(err) {
			return SetLastViewedCardgroupOutcome{}, err
		}
		return SetLastViewedCardgroupOutcome{}, eris.Wrap(err, "usecase: last viewed cardgroup: refetch own user row")
	}
	return SetLastViewedCardgroupOutcome{User: user}, nil
}
