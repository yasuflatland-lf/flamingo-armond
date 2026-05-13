package gqlerr_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/rotisserie/eris"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"backend/internal/gqlerr"
)

func extString(t *testing.T, e *gqlerror.Error, key string) string {
	t.Helper()
	v, _ := e.Extensions[key].(string)
	return v
}

func TestUnauthenticated(t *testing.T) {
	t.Parallel()

	got := gqlerr.Unauthenticated()

	if got.Message != "unauthenticated" {
		t.Errorf("Message = %q, want %q", got.Message, "unauthenticated")
	}
	if code := extString(t, got, "code"); code != "UNAUTHENTICATED" {
		t.Errorf("Extensions[code] = %q, want %q", code, "UNAUTHENTICATED")
	}
}

func TestBadUserInput(t *testing.T) {
	t.Parallel()

	got := gqlerr.BadUserInput("displayName", "too long")

	if got.Message != "too long" {
		t.Errorf("Message = %q, want %q", got.Message, "too long")
	}
	if code := extString(t, got, "code"); code != "BAD_USER_INPUT" {
		t.Errorf("Extensions[code] = %q, want %q", code, "BAD_USER_INPUT")
	}
	if field := extString(t, got, "field"); field != "displayName" {
		t.Errorf("Extensions[field] = %q, want %q", field, "displayName")
	}
}

func TestBadUserInputWithExtensions(t *testing.T) {
	t.Parallel()

	t.Run("merges extra keys", func(t *testing.T) {
		t.Parallel()
		got := gqlerr.BadUserInputWithExtensions("front", "duplicate card", map[string]any{
			"reason":         "CARD_DUPLICATE_FRONT",
			"existingCardId": "abc-123",
		})

		if got.Message != "duplicate card" {
			t.Errorf("Message = %q, want %q", got.Message, "duplicate card")
		}
		if code := extString(t, got, "code"); code != "BAD_USER_INPUT" {
			t.Errorf("Extensions[code] = %q, want %q", code, "BAD_USER_INPUT")
		}
		if field := extString(t, got, "field"); field != "front" {
			t.Errorf("Extensions[field] = %q, want %q", field, "front")
		}
		if reason := extString(t, got, "reason"); reason != "CARD_DUPLICATE_FRONT" {
			t.Errorf("Extensions[reason] = %q, want %q", reason, "CARD_DUPLICATE_FRONT")
		}
		if id := extString(t, got, "existingCardId"); id != "abc-123" {
			t.Errorf("Extensions[existingCardId] = %q, want %q", id, "abc-123")
		}
	})

	t.Run("ignores reserved code override", func(t *testing.T) {
		t.Parallel()
		got := gqlerr.BadUserInputWithExtensions("front", "duplicate card", map[string]any{
			"code": "OVERRIDE",
		})

		if code := extString(t, got, "code"); code != "BAD_USER_INPUT" {
			t.Errorf("Extensions[code] = %q, want %q (override must be ignored)", code, "BAD_USER_INPUT")
		}
	})

	t.Run("ignores reserved field override", func(t *testing.T) {
		t.Parallel()
		got := gqlerr.BadUserInputWithExtensions("front", "duplicate card", map[string]any{
			"field": "hijacked",
		})

		if field := extString(t, got, "field"); field != "front" {
			t.Errorf("Extensions[field] = %q, want %q (override must be ignored)", field, "front")
		}
	})
}

func TestInternal_message(t *testing.T) {
	t.Parallel()

	got := gqlerr.Internal(context.Background(), errors.New("boom"))

	if got.Message != "internal server error" {
		t.Errorf("Message = %q, want %q", got.Message, "internal server error")
	}
	if code := extString(t, got, "code"); code != "INTERNAL" {
		t.Errorf("Extensions[code] = %q, want %q", code, "INTERNAL")
	}
	if strings.Contains(got.Message, "boom") {
		t.Errorf("client message must not expose underlying error, got %q", got.Message)
	}
}

func TestInternal_logsError(t *testing.T) {
	// Not parallel: mutates the global slog default.
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	gqlerr.Internal(context.Background(), errors.New("boom"))

	if !strings.Contains(buf.String(), "boom") {
		t.Errorf("expected log to contain %q, got %q", "boom", buf.String())
	}
}

func TestIsCode_true(t *testing.T) {
	t.Parallel()

	if !gqlerr.IsCode(gqlerr.Unauthenticated(), gqlerr.CodeUnauthenticated) {
		t.Error("IsCode should return true for matching code")
	}
}

func TestIsCode_plainError(t *testing.T) {
	t.Parallel()

	if gqlerr.IsCode(errors.New("plain"), gqlerr.CodeUnauthenticated) {
		t.Error("IsCode should return false for non-gqlerror")
	}
}

func TestIsCode_NilExtensions(t *testing.T) {
	t.Parallel()

	if gqlerr.IsCode(&gqlerror.Error{Message: "x"}, gqlerr.CodeInternal) {
		t.Error("IsCode should return false when Extensions is nil")
	}
}

func TestIsCode_EmptyCode(t *testing.T) {
	t.Parallel()

	if gqlerr.IsCode(gqlerr.Unauthenticated(), gqlerr.Code("")) {
		t.Error("IsCode should return false for empty Code")
	}
}

func TestInternal_EmitsErrorChain(t *testing.T) {
	buf := &bytes.Buffer{}
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(slog.NewJSONHandler(buf, nil)))

	gqlerr.Internal(context.Background(), eris.New("boom"))

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("decode log record: %v (raw: %s)", err, buf.String())
	}
	if rec["level"] != "ERROR" {
		t.Errorf("expected level=ERROR, got %v", rec["level"])
	}
	if rec["msg"] != "internal error" {
		t.Errorf("expected msg='internal error', got %v", rec["msg"])
	}
	chain, ok := rec["error_chain"]
	if !ok {
		t.Fatalf("error_chain missing: %v", rec)
	}
	chainMap, ok := chain.(map[string]any)
	if !ok {
		t.Fatalf("expected error_chain to be a JSON object, got %T", chain)
	}
	if _, hasRoot := chainMap["root"]; !hasRoot {
		t.Errorf("expected error_chain.root, got %v", chainMap)
	}
}

func TestNewForbidden(t *testing.T) {
	t.Parallel()

	got := gqlerr.NewForbidden("access denied")

	if got.Message != "access denied" {
		t.Errorf("Message = %q, want %q", got.Message, "access denied")
	}
	if code := extString(t, got, "code"); code != "FORBIDDEN" {
		t.Errorf("Extensions[code] = %q, want %q", code, "FORBIDDEN")
	}
}

func TestIsCode_Forbidden(t *testing.T) {
	t.Parallel()

	if !gqlerr.IsCode(gqlerr.NewForbidden("x"), gqlerr.CodeForbidden) {
		t.Error("IsCode should return true for FORBIDDEN code")
	}
}

func TestInternal_VariadicAttrs(t *testing.T) {
	// Not parallel: mutates the global slog default.
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	got := gqlerr.Internal(
		context.Background(),
		eris.New("test error"),
		slog.String("user_id", "u1"),
		slog.String("cardgroup_id", "cg1"),
	)

	if got.Extensions["code"] != "INTERNAL" {
		t.Errorf("Extensions[code] = %v, want %q", got.Extensions["code"], "INTERNAL")
	}

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("decode log record: %v (raw: %s)", err, buf.String())
	}
	if rec["user_id"] != "u1" {
		t.Errorf("log record user_id = %v, want %q", rec["user_id"], "u1")
	}
	if rec["cardgroup_id"] != "cg1" {
		t.Errorf("log record cardgroup_id = %v, want %q", rec["cardgroup_id"], "cg1")
	}
	if rec["level"] != "ERROR" {
		t.Errorf("log record level = %v, want %q", rec["level"], "ERROR")
	}
	chain, ok := rec["error_chain"].(map[string]any)
	if !ok {
		t.Fatalf("error_chain is not a JSON object: %T", rec["error_chain"])
	}
	root, hasRoot := chain["root"].(map[string]any)
	if !hasRoot {
		t.Error("error_chain must have root entry (got external-only shape; stub may be using stdlib errors)")
	}
	if root != nil {
		if stack, _ := root["stack"].([]any); len(stack) == 0 {
			t.Error("error_chain.root.stack must contain at least one frame")
		}
	}
}

func TestCancelled(t *testing.T) {
	t.Parallel()

	// nil err path: must not log, must return the typed gqlerror shape.
	got := gqlerr.Cancelled(context.Background(), nil)

	if got.Message != "request cancelled" {
		t.Errorf("Message = %q, want %q", got.Message, "request cancelled")
	}
	if code := extString(t, got, "code"); code != "CANCELLED" {
		t.Errorf("Extensions[code] = %q, want %q", code, "CANCELLED")
	}
	if !gqlerr.IsCode(got, gqlerr.CodeCancelled) {
		t.Error("IsCode should return true for CANCELLED code")
	}

	// Non-nil err path: returned shape must match regardless of logging.
	gotWithErr := gqlerr.Cancelled(context.Background(), context.Canceled)
	if gotWithErr.Message != "request cancelled" {
		t.Errorf("Message = %q, want %q", gotWithErr.Message, "request cancelled")
	}
	if code := extString(t, gotWithErr, "code"); code != "CANCELLED" {
		t.Errorf("Extensions[code] = %q, want %q", code, "CANCELLED")
	}
}

func TestRecoverFunc(t *testing.T) {
	// Not parallel: mutates the global slog default.
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	ctx := context.Background()
	raw := gqlerr.RecoverFunc(ctx, "deliberate panic")

	if raw == nil {
		t.Fatal("RecoverFunc returned nil")
	}
	got, ok := raw.(*gqlerror.Error)
	if !ok {
		t.Fatalf("RecoverFunc returned %T, want *gqlerror.Error", raw)
	}
	if code := extString(t, got, "code"); code != "INTERNAL" {
		t.Errorf("extensions.code = %q, want INTERNAL", code)
	}
	if got.Message != "internal server error" {
		t.Errorf("Message = %q, want %q", got.Message, "internal server error")
	}
}
