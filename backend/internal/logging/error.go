// Package logging centralizes structured error logging on top of slog.
//
// LogError is the ERROR-level entry point used by the GraphQL boundary
// (gqlerr.Internal) and main()'s terminal log so every error event the
// backend emits at ERROR level carries the same shape: a human-readable
// msg and the eris error chain attached as the "error_chain" attribute.
//
// Non-ERROR sites that need the same attribute (e.g. auth.reject's Warn
// log for client-side rejections) attach eris.ToJSON(err, true) to the
// "error_chain" key directly so log shape stays consistent.
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
