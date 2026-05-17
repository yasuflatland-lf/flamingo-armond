package resolver_test

import (
	"io"
	"log/slog"
)

// newDiscardLogger returns a *slog.Logger that discards all output. Use it to
// satisfy usecase constructors that require a non-nil logger in unit tests.
func newDiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
