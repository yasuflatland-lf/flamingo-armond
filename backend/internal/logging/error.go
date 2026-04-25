// Package logging centralizes structured error logging on top of slog.
//
// LogError is the single entry point used by the GraphQL boundary, the auth
// middleware, and main() so that every "error event" emitted by the backend
// carries the same shape: ERROR level, a human-readable msg, and the eris
// error chain attached as the "error_chain" attribute.
package logging

import (
	"context"
	"log/slog"

	"github.com/rotisserie/eris"
)

// LogError records err at slog.LevelError using the provided logger.
//
// When err is nil the call is a no-op so callers do not need to guard the
// happy path. The error chain (root + wrap frames + stack) is attached as a
// structured attribute named "error_chain" so JSON log aggregators can index
// it without re-parsing free-form strings.
func LogError(ctx context.Context, logger *slog.Logger, msg string, err error, attrs ...slog.Attr) {
	if err == nil {
		return
	}
	if logger == nil {
		logger = slog.Default()
	}
	all := []slog.Attr{slog.Any("error_chain", eris.ToJSON(err, true))}
	all = append(all, attrs...)
	logger.LogAttrs(ctx, slog.LevelError, msg, all...)
}
