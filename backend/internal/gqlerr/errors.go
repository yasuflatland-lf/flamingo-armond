package gqlerr

import (
	"context"
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

// Internal logs err at ERROR level and returns a generic INTERNAL gqlerror. The
// optional attrs are attached to the log line only — the wire response is always
// the same {code: INTERNAL, message: "internal server error"} shape. Use attrs
// to attach triage-relevant context (e.g. entity IDs) that must not leak to the
// client but helps operators distinguish a known race from a real regression.
func Internal(ctx context.Context, err error, attrs ...slog.Attr) *gqlerror.Error {
	logging.LogError(ctx, slog.Default(), "internal error", err, attrs...)
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
