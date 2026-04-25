package logging

import (
	"context"
	"log/slog"
)

// ContextLookup retrieves a string value from a context.
// Using a function rather than importing middleware directly avoids an import cycle.
type ContextLookup func(context.Context) string

// ContextHandler is an slog.Handler that enriches every log record with
// values extracted from the record's context via a ContextLookup function.
type ContextHandler struct {
	inner  slog.Handler
	lookup ContextLookup
}

func NewContextHandler(inner slog.Handler, lookup ContextLookup) *ContextHandler {
	return &ContextHandler{inner: inner, lookup: lookup}
}

func (h *ContextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	if h.lookup != nil {
		if id := h.lookup(ctx); id != "" {
			r.AddAttrs(slog.String("request_id", id))
		}
	}
	return h.inner.Handle(ctx, r)
}

func (h *ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ContextHandler{inner: h.inner.WithAttrs(attrs), lookup: h.lookup}
}

func (h *ContextHandler) WithGroup(name string) slog.Handler {
	return &ContextHandler{inner: h.inner.WithGroup(name), lookup: h.lookup}
}
