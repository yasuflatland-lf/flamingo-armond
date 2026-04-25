package logging

import (
	"context"
	"log/slog"
)

// ContextLookup is a function that retrieves a string value from a context.
// Passing the lookup function rather than importing the middleware package
// directly avoids an import cycle between logging and middleware.
type ContextLookup func(context.Context) string

// ContextHandler is an slog.Handler that enriches every log record with
// values extracted from the record's context via a `ContextLookup` function.
// In practice it is used to attach the current request_id to
// every log line automatically.
type ContextHandler struct {
	inner  slog.Handler
	lookup ContextLookup
}

// NewContextHandler wraps inner with a ContextHandler that calls lookup on
// every Handle invocation. When lookup returns a non-empty string the value
// is added to the record as the "request_id" attribute before delegating to
// inner.
func NewContextHandler(inner slog.Handler, lookup ContextLookup) *ContextHandler {
	return &ContextHandler{inner: inner, lookup: lookup}
}

// Enabled reports whether the inner handler is enabled for the given level.
func (h *ContextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle enriches r with the request_id extracted from ctx (when non-empty)
// and then forwards the record to the inner handler.
func (h *ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	if h.lookup != nil {
		if id := h.lookup(ctx); id != "" {
			r.AddAttrs(slog.String("request_id", id))
		}
	}
	return h.inner.Handle(ctx, r)
}

// WithAttrs returns a new ContextHandler whose inner handler has the given
// attributes pre-applied. The lookup function is preserved.
func (h *ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ContextHandler{inner: h.inner.WithAttrs(attrs), lookup: h.lookup}
}

// WithGroup returns a new ContextHandler whose inner handler groups subsequent
// attributes under name. The lookup function is preserved.
func (h *ContextHandler) WithGroup(name string) slog.Handler {
	return &ContextHandler{inner: h.inner.WithGroup(name), lookup: h.lookup}
}
