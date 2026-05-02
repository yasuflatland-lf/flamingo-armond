package usecase

import (
	"context"
	"errors"

	"github.com/rotisserie/eris"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/gqlerr"
	"backend/internal/repository"
)

// LastViewedCardgroupUsecase records the cardgroup the authenticated caller
// most recently viewed on /learn. The mutation is idempotent and ownership-
// checked at the repository layer (single SQL statement, no TOCTOU window the
// usecase needs to widen).
type LastViewedCardgroupUsecase interface {
	Set(ctx context.Context, cardgroupID string) (*domain.User, error)
}

// lastViewedCardgroupRepo is the narrow repository surface consumed by
// LastViewedCardgroupUsecase. Declared package-private so test doubles can
// implement only the two methods this usecase exercises.
type lastViewedCardgroupRepo interface {
	SetLastViewedCardgroup(ctx context.Context, userID, cardgroupID string) error
	FindByID(ctx context.Context, id string) (*domain.User, error)
}

type lastViewedCardgroupUsecase struct {
	users lastViewedCardgroupRepo
}

// NewLastViewedCardgroup is the production constructor. Tests should prefer
// NewLastViewedCardgroupWithDeps to inject a narrow stub.
func NewLastViewedCardgroup(users repository.UserRepository) LastViewedCardgroupUsecase {
	return &lastViewedCardgroupUsecase{users: users}
}

// NewLastViewedCardgroupWithDeps accepts the narrow interface for tests;
// production code must use NewLastViewedCardgroup.
func NewLastViewedCardgroupWithDeps(users lastViewedCardgroupRepo) LastViewedCardgroupUsecase {
	return &lastViewedCardgroupUsecase{users: users}
}

// Set records cardgroupID as the caller's most recently viewed cardgroup and
// returns the refreshed user row. Authorization rules:
//
//   - Anonymous (no auth context) → UNAUTHENTICATED.
//   - Cardgroup not owned OR cardgroup does not exist → BAD_USER_INPUT(cardgroupId).
//     The same code is returned for both cases so existence of other users'
//     cardgroups is not leaked through the error shape.
//   - Owned, existing cardgroup → success.
//
// Concurrency: a cardgroup deleted between the EXISTS check and the UPDATE
// surfaces as a 23503 FK violation, which the repository classifies into
// ErrCardgroupNotFound so the caller receives BAD_USER_INPUT rather than
// INTERNAL.
func (u *lastViewedCardgroupUsecase) Set(ctx context.Context, cardgroupID string) (*domain.User, error) {
	caller := auth.UserFrom(ctx)
	if caller == nil || caller.Sub == "" {
		return nil, gqlerr.Unauthenticated()
	}

	if err := u.users.SetLastViewedCardgroup(ctx, caller.Sub, cardgroupID); err != nil {
		switch {
		case errors.Is(err, repository.ErrCardgroupNotFound):
			return nil, gqlerr.BadUserInput("cardgroupId", "cardgroup not found or not owned")
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			return nil, gqlerr.Cancelled(ctx, err)
		default:
			return nil, gqlerr.Internal(ctx, eris.Wrap(err, "usecase: set last viewed cardgroup"))
		}
	}

	user, err := u.users.FindByID(ctx, caller.Sub)
	if err != nil {
		// ErrNotFound here means the user row vanished between the UPDATE and
		// the refetch — only possible if the auth.users row was deleted
		// concurrently. Treated as INTERNAL like any other refetch failure,
		// with the wrapped sentinel preserved for log correlation.
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, gqlerr.Cancelled(ctx, err)
		}
		return nil, gqlerr.Internal(ctx, eris.Wrap(err, "usecase: set last viewed cardgroup: refetch"))
	}
	return user, nil
}
