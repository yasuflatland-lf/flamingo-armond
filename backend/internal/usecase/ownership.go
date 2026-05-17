package usecase

import (
	"context"
	"errors"

	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/usecase/ucerr"

	"github.com/rotisserie/eris"
)

// CardgroupOwnershipFinder is the narrow repo surface ownership checks need.
type CardgroupOwnershipFinder interface {
	FindByID(ctx context.Context, id string) (*domain.Cardgroup, error)
}

// authorizeCardgroup verifies that userID owns the cardgroup identified by id.
// When missingAsBadInput is true, a not-found cardgroup becomes
// ucerr.NewValidationError("cardgroupId", "cardgroup not found"). When false,
// a not-found cardgroup becomes ucerr.ErrUnauthenticated (the existing
// behaviour callers of CardUsecase relied on when the ID had already been
// validated upstream). Pass true at create-time when the input may carry a
// stale cardgroup ID; pass false at update/delete-time where the ID was
// already authorized.
func authorizeCardgroup(
	ctx context.Context,
	repo CardgroupOwnershipFinder,
	id, userID string,
	missingAsBadInput bool,
) error {
	cg, err := repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			if missingAsBadInput {
				return ucerr.NewValidationError("cardgroupId", "cardgroup not found")
			}
			return ucerr.ErrUnauthenticated
		}
		return eris.Wrap(err, "usecase: authorize cardgroup: find by id")
	}
	if !cg.IsOwnedBy(userID) {
		return ucerr.ErrUnauthenticated
	}
	return nil
}
