package logging_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"backend/internal/logging"
)

// fixedLookup returns a ContextLookup that always returns the given value,
// simulating middleware.RequestIDFromContext for a context that carries an ID.
func fixedLookup(id string) logging.ContextLookup {
	return func(_ context.Context) string { return id }
}

// emptyLookup is a ContextLookup that always returns "", simulating a context
// that has no request ID stored.
func emptyLookup(_ context.Context) string { return "" }

// newBufLogger builds a *slog.Logger backed by a ContextHandler wrapping a
// JSON handler that writes to buf.
func newBufLogger(buf *bytes.Buffer, lookup logging.ContextLookup) *slog.Logger {
	inner := slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	return slog.New(logging.NewContextHandler(inner, lookup))
}

// decodeJSON unmarshals the first JSON object in buf.
func decodeJSON(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("JSON decode failed: %v\nraw: %s", err, buf.String())
	}
	return m
}

// TestContextHandler_RequestIDPresentInContext verifies that when the lookup
// returns a non-empty ID, the log record contains "request_id".
func TestContextHandler_RequestIDPresentInContext(t *testing.T) {
	const wantID = "test-request-id-001"
	buf := &bytes.Buffer{}
	logger := newBufLogger(buf, fixedLookup(wantID))

	logger.InfoContext(context.Background(), "hello")

	rec := decodeJSON(t, buf)
	got, ok := rec["request_id"]
	if !ok {
		t.Fatalf("expected request_id attribute in log record, got %v", rec)
	}
	if got != wantID {
		t.Errorf("request_id: want %q, got %q", wantID, got)
	}
}

// TestContextHandler_NoRequestIDInBareCtx verifies that when the lookup
// returns an empty string, no "request_id" attribute is added to the record.
func TestContextHandler_NoRequestIDInBareCtx(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := newBufLogger(buf, emptyLookup)

	logger.InfoContext(context.Background(), "hello")

	rec := decodeJSON(t, buf)
	if _, ok := rec["request_id"]; ok {
		t.Errorf("expected no request_id attribute, but found one: %v", rec["request_id"])
	}
}

// TestContextHandler_WithAttrsAndWithGroupPreserveRequestID verifies that
// loggers derived via WithAttrs and WithGroup still attach request_id.
func TestContextHandler_WithAttrsAndWithGroupPreserveRequestID(t *testing.T) {
	const wantID = "propagated-id-42"

	t.Run("WithAttrs", func(t *testing.T) {
		buf := &bytes.Buffer{}
		logger := newBufLogger(buf, fixedLookup(wantID))
		// Derive a child logger with an extra static attribute.
		child := logger.With("component", "test")
		child.InfoContext(context.Background(), "via with-attrs")

		rec := decodeJSON(t, buf)
		if got, ok := rec["request_id"]; !ok || got != wantID {
			t.Errorf("WithAttrs: request_id want %q, got %v (present=%v)", wantID, got, ok)
		}
		if rec["component"] != "test" {
			t.Errorf("WithAttrs: expected component=test, got %v", rec["component"])
		}
	})

	t.Run("WithGroup", func(t *testing.T) {
		buf := &bytes.Buffer{}
		inner := slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
		base := logging.NewContextHandler(inner, fixedLookup(wantID))
		// WithGroup wraps the inner handler's subsequent attributes under "grp".
		// request_id is added via r.AddAttrs inside Handle, so it is emitted
		// under the group key in the JSON output.
		grouped := base.WithGroup("grp")
		logger := slog.New(grouped)
		logger.InfoContext(context.Background(), "via with-group", slog.String("k", "v"))

		rec := decodeJSON(t, buf)
		// After WithGroup("grp"), the JSON handler nests attrs under "grp".
		// request_id appears inside that object because AddAttrs runs after grouping.
		grpObj, ok := rec["grp"].(map[string]any)
		if !ok {
			t.Fatalf("WithGroup: expected 'grp' key to be a JSON object, got %T (%v)", rec["grp"], rec)
		}
		if got, present := grpObj["request_id"]; !present || got != wantID {
			t.Errorf("WithGroup: request_id want %q inside grp, got %v (present=%v)", wantID, got, present)
		}
	})
}
