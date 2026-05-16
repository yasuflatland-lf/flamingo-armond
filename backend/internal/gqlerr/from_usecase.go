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
// error with eris.Wrap without breaking classification. Logger DI inside the
// usecase layer is intentionally out of scope here — this helper inherits
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
	if ve, ok := errors.AsType[*ucerr.ValidationError](err); ok {
		return BadUserInput(ve.Field, ve.Message)
	}
	if fe, ok := errors.AsType[*ucerr.ForbiddenError](err); ok {
		return NewForbidden(fe.Message)
	}
	return Internal(ctx, err)
}
