package usecase

import (
	"fmt"

	"backend/internal/domain"
	"backend/internal/usecase/ucerr"
)

// recoverDuplicateFront performs the Postgres 23505 duplicate-front recovery shared
// by the user-card (card.go) and master-card (master_card.go) create paths. The caller
// has already matched repository.ErrCardDuplicateFront; this helper runs the follow-up
// lookup of the colliding row (supplied as a closure so each caller uses its own
// aggregate-specific repository method and conflict key) and returns the existing card's
// identity as a *DuplicateCardInfo. The duplicate is surfaced as data, not an error, so
// the resolver maps it to the *DuplicateFrontError union variant.
//
// A failed re-lookup is surfaced as an error: context-cancellation identity is pinned —
// the bare context.Canceled / context.DeadlineExceeded is returned unwrapped, so
// errors.Is(err, context.Canceled) holds AND err == context.Canceled (the identity is
// not lost inside an eris chain). Any other lookup failure is wrapped with the
// caller-supplied wrapPrefix. Per the error-wrapping rule, this shared helper never
// hardcodes a fixed layer prefix — the prefix travels from the caller so the error_chain
// names the file that performed the operation. The lookup may race with a concurrent
// delete (the duplicate row vanished between the failed INSERT and this SELECT) or fail
// for an unrelated DB reason; either way it is INTERNAL so the client can retry.
func recoverDuplicateFront(
	lookup func() (existingID, existingBack string, err error),
	wrapPrefix string,
) (*DuplicateCardInfo, error) {
	existingID, existingBack, err := lookup()
	if err != nil {
		return nil, wrapInfraErr(err, wrapPrefix)
	}
	return &DuplicateCardInfo{
		ExistingID:   existingID,
		ExistingBack: existingBack,
	}, nil
}

// stageCardText validates one front/back field for the card / master-card update paths
// and stages the parsed value into the caller's repository patch. It collapses the four
// near-identical staging blocks (user-card front/back, master-card front/back) into a
// single validate-then-apply pipeline.
//
// in is the caller's optional input pointer; nil means "leave unchanged" and the helper
// returns (nil, nil, nil). requiredErr / tooLongErr are the field-specific domain
// sentinels passed to ParseCardText. apply runs the aggregate mutation
// (existing.UpdateFront / staged.UpdateBack) and returns the resulting persisted string;
// the caller wraps any aggregate error with its own two-segment prefix inside the closure
// (error-wrapping rule: shared helpers take the caller prefix, never hardcode it). The
// aggregate mutations reject only the zero CardText (defense-in-depth); ParseCardText has
// already enforced the length/non-empty invariants, so an apply error is a programmer-error
// INTERNAL, not bad user input — hence the closure wraps with eris rather than translating.
//
// Exactly one of the three return values is non-nil on any non-nil-input call:
//   - (staged, nil, nil)  success: caller assigns patch.Front/Back = staged.
//   - (nil, info, nil)    the field failed validation: caller returns its
//     Outcome{Validation: info} data result (business failure as data, not error).
//   - (nil, nil, err)     a hard error (non-validation ParseCardText failure, or an apply
//     failure): caller returns Outcome{}, err.
func stageCardText(
	in *string,
	requiredErr, tooLongErr error,
	apply func(domain.CardText) (string, error),
) (*string, *InputValidationInfo, error) {
	if in == nil {
		return nil, nil, nil
	}
	text, err := domain.ParseCardText(*in, requiredErr, tooLongErr)
	if err != nil {
		info, liftErr := liftValidationErr(translateCardErr(err))
		if liftErr != nil {
			return nil, nil, liftErr
		}
		return nil, info, nil
	}
	s, err := apply(text)
	if err != nil {
		return nil, nil, err
	}
	return &s, nil, nil
}

// checkBulkDeleteCap enforces the shared per-call id cap for the two bulk-delete paths
// (cardUsecase.BulkDelete, masterCardUsecase.DeleteMasterCards). It returns a
// BAD_USER_INPUT validation error on "ids" when the batch exceeds maxBulkDelete, and nil
// otherwise. The empty-slice short-circuit stays at each caller because the follow-on
// action differs (a transaction vs. a direct DeleteMany) even though both return (0, nil).
func checkBulkDeleteCap(ids []string) error {
	if len(ids) > maxBulkDelete {
		return ucerr.NewValidationError("ids", fmt.Sprintf("at most %d ids per call", maxBulkDelete))
	}
	return nil
}
