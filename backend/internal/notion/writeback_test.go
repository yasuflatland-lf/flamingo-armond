package notion

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"backend/internal/domain"
)

type writebackAppendCall struct {
	pageID string
	text   string
	ctxErr error
}

type stubParagraphAppender struct {
	mu     sync.Mutex
	calls  []writebackAppendCall
	err    error
	called chan struct{}
}

func (s *stubParagraphAppender) AppendParagraph(ctx context.Context, pageID, text string) error {
	s.mu.Lock()
	s.calls = append(s.calls, writebackAppendCall{
		pageID: pageID,
		text:   text,
		ctxErr: ctx.Err(),
	})
	s.mu.Unlock()
	if s.called != nil {
		close(s.called)
	}
	return s.err
}

func TestCardWritebacker_OnCardCreated_AppendsParagraphWithDetachedContext(t *testing.T) {
	t.Parallel()

	appender := &stubParagraphAppender{called: make(chan struct{})}
	writeback := NewCardWritebacker(appender, "page-xyz", slog.New(slog.DiscardHandler))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	writeback.OnCardCreated(ctx, &domain.Card{
		ID:          "card-1",
		CardgroupID: "cg-1",
		Front:       domain.CardText("front"),
		Back:        domain.CardText("back"),
	})

	select {
	case <-appender.called:
	case <-time.After(2 * time.Second):
		t.Fatal("AppendParagraph was not called")
	}

	appender.mu.Lock()
	calls := append([]writebackAppendCall(nil), appender.calls...)
	appender.mu.Unlock()
	if len(calls) != 1 {
		t.Fatalf("expected 1 append call, got %d", len(calls))
	}
	if calls[0].pageID != "page-xyz" || calls[0].text != "front back" {
		t.Fatalf("unexpected append call: %+v", calls[0])
	}
	if calls[0].ctxErr != nil {
		t.Fatalf("append context should be detached from canceled request context, got %v", calls[0].ctxErr)
	}
}

func TestCardWritebacker_OnCardUpdated_AppendsParagraph(t *testing.T) {
	t.Parallel()

	appender := &stubParagraphAppender{called: make(chan struct{})}
	writeback := NewCardWritebacker(appender, "page-xyz", slog.New(slog.DiscardHandler))

	writeback.OnCardUpdated(context.Background(), &domain.Card{
		ID:          "card-1",
		CardgroupID: "cg-1",
		Front:       domain.CardText("new"),
		Back:        domain.CardText("value"),
	})

	select {
	case <-appender.called:
	case <-time.After(2 * time.Second):
		t.Fatal("AppendParagraph was not called")
	}

	appender.mu.Lock()
	calls := append([]writebackAppendCall(nil), appender.calls...)
	appender.mu.Unlock()
	if len(calls) != 1 {
		t.Fatalf("expected 1 append call, got %d", len(calls))
	}
	if calls[0].text != "new value" {
		t.Fatalf("append text = %q, want %q", calls[0].text, "new value")
	}
}

type signalWriter struct {
	buf  bytes.Buffer
	done chan struct{}
	once sync.Once
}

func (w *signalWriter) Write(p []byte) (int, error) {
	n, err := w.buf.Write(p)
	w.once.Do(func() { close(w.done) })
	return n, err
}

func TestCardWritebacker_ErrorLogsCardgroupID(t *testing.T) {
	t.Parallel()

	sw := &signalWriter{done: make(chan struct{})}
	logger := slog.New(slog.NewJSONHandler(sw, &slog.HandlerOptions{Level: slog.LevelDebug}))
	appender := &stubParagraphAppender{
		err:    errors.New("notion unavailable"),
		called: make(chan struct{}),
	}
	writeback := NewCardWritebacker(appender, "page-log", logger)

	writeback.OnCardCreated(context.Background(), &domain.Card{
		ID:          "card-log",
		CardgroupID: "cg-log",
		Front:       domain.CardText("front"),
		Back:        domain.CardText("back"),
	})

	select {
	case <-sw.done:
	case <-time.After(2 * time.Second):
		t.Fatal("writeback failure was not logged")
	}

	records := decodeWritebackJSONRecords(t, sw.buf.Bytes())
	var warnRec map[string]any
	for _, rec := range records {
		if rec["msg"] == "card create: notion writeback failed" {
			warnRec = rec
			break
		}
	}
	if warnRec == nil {
		t.Fatalf("missing writeback warning log; output:\n%s", sw.buf.String())
	}
	if warnRec["cardgroup_id"] != "cg-log" {
		t.Fatalf("cardgroup_id = %v, want cg-log; record=%v", warnRec["cardgroup_id"], warnRec)
	}
	if warnRec["page_id"] != "page-log" {
		t.Fatalf("page_id = %v, want page-log; record=%v", warnRec["page_id"], warnRec)
	}
	if warnRec["card_id"] != "card-log" {
		t.Fatalf("card_id = %v, want card-log; record=%v", warnRec["card_id"], warnRec)
	}
}

func decodeWritebackJSONRecords(t *testing.T, data []byte) []map[string]any {
	t.Helper()
	var records []map[string]any
	for _, line := range bytes.Split(bytes.TrimSpace(data), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal(line, &rec); err != nil {
			t.Fatalf("decode log line %q: %v", line, err)
		}
		records = append(records, rec)
	}
	return records
}
