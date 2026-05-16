// Package usecase re-exports the usecase-layer error types defined in the
// internal sub-package backend/internal/usecase/ucerr.
//
// This file re-exports the typed errors defined in ./ucerr as aliases so
// callers can write *usecase.ValidationError interchangeably with
// *ucerr.ValidationError. The CI gate at .github/workflows/backend.yml
// forbids gqlerr imports in this package; placing the type definitions in
// the ucerr sub-package keeps that boundary mechanical to maintain.
// Aliases preserve errors.Is / errors.As behaviour across both names.
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
