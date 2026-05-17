package resolver_test

import "log/slog"

// newDiscardLogger returns a *slog.Logger backed by slog.DiscardHandler. Use it
// to satisfy usecase constructors that require a non-nil logger in unit tests.
func newDiscardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}
