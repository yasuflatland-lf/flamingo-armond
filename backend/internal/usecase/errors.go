// Package usecase re-exports the usecase-layer error types defined in the
// internal sub-package backend/internal/usecase/ucerr.
//
// The sub-package exists to break the import cycle that would otherwise occur
// because the production usecase code still imports backend/internal/gqlerr
// (issue #158 removes those imports). Until then, gqlerr.FromUsecaseError
// imports `usecase/ucerr` directly; resolvers and tests may write either
// `*usecase.ValidationError` or `*ucerr.ValidationError` — they are the same
// type (Go alias), and errors.Is / errors.As succeed across both names.
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
