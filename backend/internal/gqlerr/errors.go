package gqlerr

import (
	"context"
	"errors"
	"log/slog"

	"github.com/vektah/gqlparser/v2/gqlerror"
)

type Code string

const (
	CodeUnauthenticated Code = "UNAUTHENTICATED"
	CodeBadUserInput    Code = "BAD_USER_INPUT"
	CodeInternal        Code = "INTERNAL"
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

func Internal(ctx context.Context, err error) *gqlerror.Error {
	slog.ErrorContext(ctx, "internal error", "error", err)
	return &gqlerror.Error{
		Message: "internal server error",
		Extensions: map[string]any{
			"code": string(CodeInternal),
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
