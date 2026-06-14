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

// authorizeCardgroupOrBadInput verifies that userID owns the cardgroup identified
// by id, treating a not-found cardgroup as a validation error against the caller-
// supplied input. Use at the boundary where id originates from untrusted user
// input (mutation arguments, list filters) and a stale id is a recoverable
// caller mistake rather than a security event.
func authorizeCardgroupOrBadInput(
	ctx context.Context,
	repo CardgroupOwnershipFinder,
	id, userID string,
) error {
	cg, err := repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ucerr.NewValidationError("cardgroupId", "cardgroup not found")
		}
		if isContextDone(err) {
			return err
		}
		return eris.Wrap(err, "usecase: authorize cardgroup: find by id")
	}
	if !cg.IsOwnedBy(domain.UserID(userID)) {
		return ucerr.ErrUnauthenticated
	}
	return nil
}

// authorizeCardgroupOrUnauthenticated verifies that userID owns the cardgroup
// identified by id, treating a not-found cardgroup as an authorization failure.
// Use deeper in the application layer where id was already validated upstream
// (e.g. resolved from a DB-fetched entity) and a missing cardgroup signals an
// authorization or consistency issue, not a caller mistake.
func authorizeCardgroupOrUnauthenticated(
	ctx context.Context,
	repo CardgroupOwnershipFinder,
	id, userID string,
) error {
	cg, err := repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ucerr.ErrUnauthenticated
		}
		if isContextDone(err) {
			return err
		}
		return eris.Wrap(err, "usecase: authorize cardgroup: find by id")
	}
	if !cg.IsOwnedBy(domain.UserID(userID)) {
		return ucerr.ErrUnauthenticated
	}
	return nil
}
