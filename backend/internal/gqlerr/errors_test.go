package gqlerr_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"backend/internal/gqlerr"
)

func TestUnauthenticated(t *testing.T) {
	t.Parallel()

	got := gqlerr.Unauthenticated()

	if got.Message != "unauthenticated" {
		t.Errorf("Message = %q, want %q", got.Message, "unauthenticated")
	}
	if code, _ := got.Extensions["code"].(string); code != "UNAUTHENTICATED" {
		t.Errorf("Extensions[code] = %q, want %q", code, "UNAUTHENTICATED")
	}
}

func TestBadUserInput(t *testing.T) {
	t.Parallel()

	got := gqlerr.BadUserInput("displayName", "too long")

	if got.Message != "too long" {
		t.Errorf("Message = %q, want %q", got.Message, "too long")
	}
	if code, _ := got.Extensions["code"].(string); code != "BAD_USER_INPUT" {
		t.Errorf("Extensions[code] = %q, want %q", code, "BAD_USER_INPUT")
	}
	if field, _ := got.Extensions["field"].(string); field != "displayName" {
		t.Errorf("Extensions[field] = %q, want %q", field, "displayName")
	}
}

func TestInternal_message(t *testing.T) {
	t.Parallel()

	orig := errors.New("boom")
	got := gqlerr.Internal(context.Background(), orig)

	if got.Message != "internal server error" {
		t.Errorf("Message = %q, want %q", got.Message, "internal server error")
	}
	if code, _ := got.Extensions["code"].(string); code != "INTERNAL" {
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
