// Package logging centralizes structured error logging on top of slog.
//
// LogError (ERROR) and LogWarn (WARN) are the two entry points the backend
// uses for events that carry an error chain. Both attach eris.ToJSON(err,
// true) under the "error_chain" attribute so JSON aggregators can index the
// chain without re-parsing free-form strings.
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
	logAt(ctx, logger, slog.LevelError, msg, err, attrs...)
}

// LogWarn records err at slog.LevelWarn using the provided logger. Same nil
// and shape semantics as LogError; intended for client-attributable failures
// (auth rejection, request cancellation) where the operator should not be
// paged but the error chain is still useful for debugging.
func LogWarn(ctx context.Context, logger *slog.Logger, msg string, err error, attrs ...slog.Attr) {
	logAt(ctx, logger, slog.LevelWarn, msg, err, attrs...)
}

func logAt(ctx context.Context, logger *slog.Logger, level slog.Level, msg string, err error, attrs ...slog.Attr) {
	if err == nil {
		return
	}
	if logger == nil {
		logger = slog.Default()
	}
	all := []slog.Attr{slog.Any("error_chain", eris.ToJSON(err, true))}
	all = append(all, attrs...)
	logger.LogAttrs(ctx, level, msg, all...)
}
