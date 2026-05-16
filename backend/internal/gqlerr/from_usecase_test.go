package gqlerr_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/rotisserie/eris"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"backend/internal/gqlerr"
	"backend/internal/usecase/ucerr"
)

// silenceLogger redirects slog.Default() to io.Discard for the duration of the
// test and restores the previous default via t.Cleanup. Must not be used with
// t.Parallel() because mutating the global slog default is a data race (see
// .claude/rules/go-library-gotchas.md § t.Parallel() + slog.SetDefault()).
func silenceLogger(t *testing.T) {
	t.Helper()
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
}

func TestFromUsecaseError(t *testing.T) {
	// Do NOT call t.Parallel() at the top level — sub-tests that silence the
	// logger mutate the global slog default and would race with parallel siblings.

	type tc struct {
		name        string
		err         error
		wantNil     bool
		wantCode    gqlerr.Code
		wantMessage string
		wantField   string // non-empty only for BAD_USER_INPUT cases
	}

	cases := []tc{
		{
			name:    "nil input returns nil",
			err:     nil,
			wantNil: true,
		},
		{
			name:        "context.Canceled returns CANCELLED",
			err:         context.Canceled,
			wantCode:    gqlerr.CodeCancelled,
			wantMessage: "request cancelled",
		},
		{
			name:        "context.DeadlineExceeded returns CANCELLED",
			err:         context.DeadlineExceeded,
			wantCode:    gqlerr.CodeCancelled,
			wantMessage: "request cancelled",
		},
		{
			name:        "eris-wrapped context.Canceled returns CANCELLED (wrap traversal)",
			err:         eris.Wrap(context.Canceled, "outer: cancel race"),
			wantCode:    gqlerr.CodeCancelled,
			wantMessage: "request cancelled",
		},
		{
			name:        "ErrUnauthenticated returns UNAUTHENTICATED",
			err:         ucerr.ErrUnauthenticated,
			wantCode:    gqlerr.CodeUnauthenticated,
			wantMessage: "unauthenticated",
		},
		{
			name:        "eris-wrapped ErrUnauthenticated returns UNAUTHENTICATED",
			err:         eris.Wrap(ucerr.ErrUnauthenticated, "outer: cardgroup"),
			wantCode:    gqlerr.CodeUnauthenticated,
			wantMessage: "unauthenticated",
		},
		{
			name:        "ValidationError returns BAD_USER_INPUT with field and message",
			err:         &ucerr.ValidationError{Field: "first", Message: "first or last must be > 0"},
			wantCode:    gqlerr.CodeBadUserInput,
			wantMessage: "first or last must be > 0",
			wantField:   "first",
		},
		{
			name:        "eris-wrapped ValidationError returns BAD_USER_INPUT (errors.As traversal)",
			err:         eris.Wrap(&ucerr.ValidationError{Field: "after", Message: "cursor not found"}, "outer: cards page"),
			wantCode:    gqlerr.CodeBadUserInput,
			wantMessage: "cursor not found",
			wantField:   "after",
		},
		{
			name:        "ForbiddenError returns FORBIDDEN with message",
			err:         &ucerr.ForbiddenError{Message: "admin only"},
			wantCode:    gqlerr.CodeForbidden,
			wantMessage: "admin only",
		},
		{
			name:        "eris-wrapped ForbiddenError returns FORBIDDEN (errors.As traversal)",
			err:         eris.Wrap(&ucerr.ForbiddenError{Message: "admin only"}, "outer: cardgroup"),
			wantCode:    gqlerr.CodeForbidden,
			wantMessage: "admin only",
		},
		{
			name:        "unknown error returns INTERNAL",
			err:         errors.New("boom"),
			wantCode:    gqlerr.CodeInternal,
			wantMessage: "internal server error",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Silence logger for cases that call gqlerr.Internal or
			// gqlerr.Cancelled, which log to slog.Default(). This must happen
			// before calling FromUsecaseError.
			silenceLogger(t)

			got := gqlerr.FromUsecaseError(context.Background(), tc.err)

			if tc.wantNil {
				if got != nil {
					t.Errorf("FromUsecaseError(nil) = %v, want nil", got)
				}
				return
			}

			if !gqlerr.IsCode(got, tc.wantCode) {
				t.Errorf("IsCode(%v) = false, want true for code %q", got, tc.wantCode)
			}

			var gqe *gqlerror.Error
			if !errors.As(got, &gqe) {
				t.Fatalf("returned error is not *gqlerror.Error: %T", got)
			}

			if gqe.Message != tc.wantMessage {
				t.Errorf("Message = %q, want %q", gqe.Message, tc.wantMessage)
			}

			if tc.wantField != "" {
				field, _ := gqe.Extensions["field"].(string)
				if field != tc.wantField {
					t.Errorf("Extensions[field] = %q, want %q", field, tc.wantField)
				}
			}
		})
	}
}
