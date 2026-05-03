package gqlerr

import (
	"context"
	"errors"
	"log/slog"

	"github.com/vektah/gqlparser/v2/gqlerror"

	"backend/internal/logging"
)

type Code string

const (
	CodeUnauthenticated Code = "UNAUTHENTICATED"
	CodeBadUserInput    Code = "BAD_USER_INPUT"
	CodeInternal        Code = "INTERNAL"
	CodeForbidden       Code = "FORBIDDEN"
	CodeCancelled       Code = "CANCELLED"
)

func Unauthenticated() *gqlerror.Error {
	return &gqlerror.Error{
		Message: "unauthenticated",
		Extensions: map[string]any{
			"code": string(CodeUnauthenticated),
		},
	}
}

func BadUserInput(field, message string) *gqlerror.Error {
	return &gqlerror.Error{
		Message: message,
		Extensions: map[string]any{
			"code":  string(CodeBadUserInput),
			"field": field,
		},
	}
}

// BadUserInputWithExtensions returns a BAD_USER_INPUT error with additional
// extensions merged into the standard {code, field} envelope. Reserved keys
// (code, field) in extra are ignored to keep the envelope stable.
func BadUserInputWithExtensions(field, message string, extra map[string]any) *gqlerror.Error {
	ext := map[string]any{
		"code":  string(CodeBadUserInput),
		"field": field,
	}
	for k, v := range extra {
		if k == "code" || k == "field" {
			continue
		}
		ext[k] = v
	}
	return &gqlerror.Error{Message: message, Extensions: ext}
}

func Internal(ctx context.Context, err error) *gqlerror.Error {
	logging.LogError(ctx, slog.Default(), "internal error", err)
	return &gqlerror.Error{
		Message: "internal server error",
		Extensions: map[string]any{
			"code": string(CodeInternal),
		},
	}
}

// NewForbidden returns a FORBIDDEN GraphQL error. The caller supplies a
// human-readable message; do not include sensitive details (e.g. "user X is
// not admin") — keep the message generic.
func NewForbidden(msg string) *gqlerror.Error {
	return &gqlerror.Error{
		Message: msg,
		Extensions: map[string]any{
			"code": string(CodeForbidden),
		},
	}
}

// Cancelled returns a typed CANCELLED gqlerror. When err is non-nil it is
// logged at WARN with the underlying cause; pass the original error from
// IsAdmin / context to preserve the eris chain. The helper covers both
// context.Canceled and context.DeadlineExceeded; callers decide which to use.
func Cancelled(ctx context.Context, err error) *gqlerror.Error {
	logging.LogWarn(ctx, slog.Default(), "request cancelled", err)
	return &gqlerror.Error{
		Message: "request cancelled",
		Extensions: map[string]any{
			"code": string(CodeCancelled),
		},
	}
}

// IsCode reports whether err is a *gqlerror.Error whose extensions.code equals
// code. An empty code never matches.
func IsCode(err error, code Code) bool {
	if code == "" {
		return false
	}
	var gqe *gqlerror.Error
	if !errors.As(err, &gqe) {
		return false
	}
	got, _ := gqe.Extensions["code"].(string)
	return got == string(code)
}
