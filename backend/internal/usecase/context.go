package usecase

import (
	"context"
	"errors"
)

// isContextDone reports whether err is context.Canceled or
// context.DeadlineExceeded. Use at usecase boundaries that must
// pass the cancellation up unchanged.
func isContextDone(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
