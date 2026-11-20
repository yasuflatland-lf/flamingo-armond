package resolver_test

import "log/slog"

// newDiscardLogger returns a *slog.Logger backed by slog.DiscardHandler. Use it
// to satisfy the usecase constructors that log.
func newDiscardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}
