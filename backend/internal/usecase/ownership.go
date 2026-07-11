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

// findOwnedCardgroup loads the cardgroup identified by id and verifies that
// userID owns it, returning the loaded aggregate on success. It is the shared
// core behind authorizeCardgroupOrBadInput / authorizeCardgroupOrUnauthenticated
// and the entity-returning entry point for callers (e.g. cardgroupUsecase.Update)
// that need the loaded row immediately after the ownership check.
//
// notFoundErr is returned when the repository reports repository.ErrNotFound:
// callers pass a validation error when id originates from untrusted input and
// ucerr.ErrUnauthenticated when a missing row signals an authorization or
// consistency issue. context.Canceled / context.DeadlineExceeded pass through
// unwrapped so the resolver can route them to Cancelled via errors.Is; any other
// infrastructure error is wrapped as INTERNAL. A found-but-foreign row always
// returns ucerr.ErrUnauthenticated; when the caller passes ucerr.ErrUnauthenticated
// as notFoundErr, missing and found-but-foreign collapse to one indistinguishable
// outcome so a stale id cannot be used as an existence oracle over another user's
// cardgroups.
func findOwnedCardgroup(
	ctx context.Context,
	repo CardgroupOwnershipFinder,
	id domain.CardgroupID,
	userID domain.UserID,
	notFoundErr error,
) (*domain.Cardgroup, error) {
	cg, err := repo.FindByID(ctx, string(id))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, notFoundErr
		}
		if isContextDone(err) {
			return nil, err
		}
		return nil, eris.Wrap(err, "usecase: authorize cardgroup: find by id")
	}
	if !cg.IsOwnedBy(userID) {
		return nil, ucerr.ErrUnauthenticated
	}
	return cg, nil
}

// authorizeCardgroupOrBadInput verifies that userID owns the cardgroup identified
// by id, treating a not-found cardgroup as a validation error against the caller-
// supplied input. Use at the boundary where id originates from untrusted user
// input (mutation arguments, list filters) and a stale id is a recoverable
// caller mistake rather than a security event. Delegates to findOwnedCardgroup
// and discards the loaded entity.
func authorizeCardgroupOrBadInput(
	ctx context.Context,
	repo CardgroupOwnershipFinder,
	id domain.CardgroupID,
	userID domain.UserID,
) error {
	_, err := findOwnedCardgroup(ctx, repo, id, userID,
		ucerr.NewValidationError("cardgroupId", "cardgroup not found"))
	return err
}

// authorizeCardgroupOrUnauthenticated verifies that userID owns the cardgroup
// identified by id, treating a not-found cardgroup as an authorization failure.
// Use deeper in the application layer where id was already validated upstream
// (e.g. resolved from a DB-fetched entity) and a missing cardgroup signals an
// authorization or consistency issue, not a caller mistake. Delegates to
// findOwnedCardgroup and discards the loaded entity.
func authorizeCardgroupOrUnauthenticated(
	ctx context.Context,
	repo CardgroupOwnershipFinder,
	id domain.CardgroupID,
	userID domain.UserID,
) error {
	_, err := findOwnedCardgroup(ctx, repo, id, userID, ucerr.ErrUnauthenticated)
	return err
}
