// Package ucerr holds the usecase-layer sentinel and structured error types.
// It is intentionally a separate sub-package so that the gqlerr package can
// import these types without creating an import cycle: gqlerr itself is
// imported by the parent usecase package.
package ucerr

import "errors"

// ErrUnauthenticated signals an unauthenticated caller from a usecase method.
// The resolver layer translates this into gqlerr.Unauthenticated() via
// gqlerr.FromUsecaseError so the wire response carries
// extensions.code = "UNAUTHENTICATED".
var ErrUnauthenticated = errors.New("usecase: not authenticated")

// ValidationError is the field-scoped, message-bearing validation failure
// returned from usecases when an input argument is invalid in a way that the
// frontend should surface against a specific input field. The pointer-receiver
// shape is required so callers can recover the value with
// errors.As(err, new(*ValidationError)) even after eris.Wrap.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return "usecase: validation: " + e.Field + ": " + e.Message
}

// ForbiddenError signals an authorized-but-not-permitted caller. The Message
// is propagated to the FORBIDDEN gqlerror's user-facing message. Same
// pointer-receiver discipline as ValidationError.
type ForbiddenError struct {
	Message string
}

func (e *ForbiddenError) Error() string {
	return "usecase: forbidden: " + e.Message
}
