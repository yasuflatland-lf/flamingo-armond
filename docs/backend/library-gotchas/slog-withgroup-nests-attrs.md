# `slog.Handler.WithGroup` nests subsequent attrs inside the group object

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

Calling `handler.WithGroup("g")` on a `slog.JSONHandler` (or any handler that
wraps one, such as `logging.ContextHandler`) causes **all** attrs added
afterward — including those injected by `Handle` via `r.AddAttrs` — to appear
under the `"g"` JSON key, not at the top level. In `logging.ContextHandler`,
`request_id` is added via `r.AddAttrs` inside `Handle`, so after
`WithGroup("grp")` the log line becomes `{"grp":{"request_id":"...","k":"v"}}`.
Log queries and tests that expect top-level `request_id` will miss it.
`backend/internal/logging/handler_test.go` (`TestContextHandler_WithAttrsAndWithGroupPreserveRequestID`)
documents and asserts this shape.
