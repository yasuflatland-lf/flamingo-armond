// Package usecase re-exports the usecase-layer error types defined in the
// internal sub-package backend/internal/usecase/ucerr.
//
// Keeping these types in a leaf sub-package preserves a one-way dependency:
// gqlerr imports ucerr (to translate via gqlerr.FromUsecaseError), while ucerr
// imports nothing from the resolver or transport layers. The parent usecase
// package re-exports these as type aliases so resolvers and tests may write
// either *usecase.ValidationError or *ucerr.ValidationError — they are the
// same type, and errors.Is / errors.As succeed across both names.
package usecase

import "backend/internal/usecase/ucerr"

// ErrUnauthenticated re-exports ucerr.ErrUnauthenticated as a single sentinel
// value. errors.Is(err, usecase.ErrUnauthenticated) and
// errors.Is(err, ucerr.ErrUnauthenticated) are equivalent.
var ErrUnauthenticated = ucerr.ErrUnauthenticated

// ValidationError is a type alias for ucerr.ValidationError so callers may
// use the shorter usecase.ValidationError name without introducing a second
// distinct type.
type ValidationError = ucerr.ValidationError

// ForbiddenError is a type alias for ucerr.ForbiddenError. See ValidationError.
type ForbiddenError = ucerr.ForbiddenError
