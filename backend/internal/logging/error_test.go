package logging_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/rotisserie/eris"

	"backend/internal/logging"
)

func newJSONLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func decodeRecord(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("decode JSON record: %v", err)
	}
	return rec
}

func TestLogError_AttachesErisChain(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := newJSONLogger(buf)

	root := eris.New("inner failure")
	wrapped := eris.Wrap(root, "outer context")

	logging.LogError(context.Background(), logger, "test event", wrapped)

	rec := decodeRecord(t, buf)
	if rec["level"] != "ERROR" {
		t.Errorf("expected level=ERROR, got %v", rec["level"])
	}
	if rec["msg"] != "test event" {
		t.Errorf("expected msg='test event', got %v", rec["msg"])
	}
	chain, ok := rec["error_chain"]
	if !ok {
		t.Fatalf("error_chain attribute missing: %v", rec)
	}
	chainMap, ok := chain.(map[string]any)
	if !ok {
		t.Fatalf("expected error_chain to be a JSON object, got %T", chain)
	}
	if _, hasRoot := chainMap["root"]; !hasRoot {
		t.Errorf("expected error_chain.root, got %v", chainMap)
	}
}

func TestLogError_PlainErrorStillLogged(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := newJSONLogger(buf)

	logging.LogError(context.Background(), logger, "plain", eris.New("oops"))
	rec := decodeRecord(t, buf)
	if rec["msg"] != "plain" {
		t.Errorf("msg mismatch: %v", rec)
	}
}

func TestLogError_NilErrorIsNoop(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := newJSONLogger(buf)

	logging.LogError(context.Background(), logger, "should not log", nil)
	if buf.Len() != 0 {
		t.Errorf("expected empty buffer for nil error, got %s", buf.String())
	}
}
