# slog context enrichment must precede the log call that announces the enrichment

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

When a middleware sets a value in the context and then logs a message about
that action, the `slog.*Context` call must come **after**
`c.SetRequest(c.Request().WithContext(ctx))`. Logging before the context is
stored means the very line that announces the event carries no `request_id` (or
other context attribute) itself. The pattern in
`backend/internal/middleware/request_id.go` — enriching the context first, then
calling `slog.WarnContext(ctx, ...)` — is the correct template for any
context-enriched slog handler.
