// Package ucerr holds the usecase-layer sentinel and structured error types
// that the resolver layer translates to wire-format GraphQL errors via
// gqlerr.FromUsecaseError. The types live in a leaf sub-package so that
// gqlerr can import them without acquiring a transitive dependency on the
// parent usecase package; the parent re-exports them as type aliases (and a re-exported sentinel var) for
// caller ergonomics.
package ucerr

import "errors"

// ErrUnauthenticated signals an unauthenticated caller from a usecase method.
// The resolver layer translates this via gqlerr.FromUsecaseError so the wire
// response carries extensions.code = "UNAUTHENTICATED".
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
// is propagated to the wire error's user-facing message field when
// FromUsecaseError translates to extensions.code = "FORBIDDEN". Same
// pointer-receiver discipline as ValidationError.
type ForbiddenError struct {
	Message string
}

func (e *ForbiddenError) Error() string {
	return "usecase: forbidden: " + e.Message
}

// NewValidationError returns a *ValidationError. The pointer return makes the
// errors.As contract impossible to break by accident: a caller writing
// ucerr.ValidationError{...} (value, no &) would silently fall through to
// gqlerr's Internal branch because errors.As(err, &ve) only matches the
// pointer form. field must be non-empty — an empty field produces
// extensions.field == "" on the wire, which the frontend cannot render
// against any input.
func NewValidationError(field, message string) *ValidationError {
	if field == "" {
		panic("ucerr.NewValidationError: field must be non-empty")
	}
	return &ValidationError{Field: field, Message: message}
}

// NewForbiddenError returns a *ForbiddenError. Same pointer-return reason as
// NewValidationError. Message is unchecked; empty messages are permitted
// because the frontend has fallback copy.
func NewForbiddenError(message string) *ForbiddenError {
	return &ForbiddenError{Message: message}
}
