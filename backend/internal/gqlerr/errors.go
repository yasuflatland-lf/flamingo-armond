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
//
// The design places variant-specific data under an extensions.reason
// sub-discriminator rather than a separate top-level code so that
// IsCode(err, CodeBadUserInput) and existing field-error UI keep working without
// modification. Reach for this helper — rather than BadUserInput — when the
// payload needs to be structurally parsed by the frontend (e.g. to surface an
// existing duplicate entity). For plain field validation messages, BadUserInput
// is sufficient.
//
// Current callers: none. Retained as a primitive for future structured
// BAD_USER_INPUT shapes; the previous caller BadUserInputCardDuplicateFront
// was removed when CardDuplicateFrontError moved to the CreateCardResult union.
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
