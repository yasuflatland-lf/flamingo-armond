package gqlerr

import (
	"context"
	"errors"

	"backend/internal/usecase/ucerr"
)

// FromUsecaseError converts a usecase-layer error into the GraphQL transport
// error.
//
// Branch order (cancel → unauth → validation → forbidden → internal):
//   - context.Canceled / context.DeadlineExceeded ⇒ Cancelled (observed earlier
//     in usecase flow than authentication; misclassifying as UNAUTHENTICATED
//     would invert the operator signal).
//   - ucerr.ErrUnauthenticated ⇒ Unauthenticated.
//   - *ucerr.ValidationError ⇒ BadUserInput, propagating Field and Message.
//   - *ucerr.ForbiddenError ⇒ NewForbidden, propagating Message.
//   - anything else ⇒ Internal (logged via gqlerr.Internal).
//
// errors.Is and errors.As are used throughout, so callers may wrap the source
// error with eris.Wrap without breaking classification. Logger DI inside
// usecase is X-7 and intentionally out of scope here — this helper inherits
// the slog.Default() behavior of gqlerr.Internal / gqlerr.Cancelled unchanged.
func FromUsecaseError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return Cancelled(ctx, err)
	}
	if errors.Is(err, ucerr.ErrUnauthenticated) {
		return Unauthenticated()
	}
	var ve *ucerr.ValidationError
	if errors.As(err, &ve) {
		return BadUserInput(ve.Field, ve.Message)
	}
	var fe *ucerr.ForbiddenError
	if errors.As(err, &fe) {
		return NewForbidden(fe.Message)
	}
	return Internal(ctx, err)
}
