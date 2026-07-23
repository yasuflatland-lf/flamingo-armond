package usecase

import (
	"context"
	"errors"

	"github.com/rotisserie/eris"
)

// isContextDone reports whether err is context.Canceled or
// context.DeadlineExceeded. Use at usecase boundaries that must
// pass the cancellation up unchanged.
func isContextDone(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// wrapInfraErr passes context.Canceled / context.DeadlineExceeded through
// unwrapped (per the pinned identity-check convention) and eris-wraps every
// other error with the caller-supplied layer prefix.
func wrapInfraErr(err error, prefix string) error {
	if isContextDone(err) {
		return err
	}
	return eris.Wrap(err, prefix)
}
