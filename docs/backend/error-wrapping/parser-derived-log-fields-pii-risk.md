# Parser-derived error messages must not leak into log fields — line + count only

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.

Parser/lexer error messages — anything ultimately constructed by a yacc-style `Error(s string)` callback — are at risk of containing fragments of the input the parser was processing when the error occurred. Today's grammar may emit only `"syntax error"`, but a future contributor adding `"unexpected token %q"` to improve error messages will silently start logging user-supplied content the moment it ships. The log site has no signal that the change is risky because the field name (`first_error_message`) was already there.

**Rule:** when logging a parser-derived `ValidationError`, include `line` and `count`, never `message`:

```go
// Good — operator can find which row failed.
logger.WarnContext(ctx, "all rows failed to parse",
    "parse_error_count", len(parseErrs),
    "first_error_line",  parseErrs[0].Line,
)

// Bad — Message may carry input fragments now or after a future grammar change.
logger.WarnContext(ctx, "all rows failed to parse",
    "first_error_message", parseErrs[0].Message,
)
```

**Why "line + count is enough":** the operator already has the source content (it lives in Notion / the database / wherever the parser read from). Knowing *which* line failed is enough to retrieve and inspect the offending row. Echoing the parser's diagnostic into the log adds no operational power and creates a slow-burning PII leak.

**Reference:** `backend/internal/usecase/notion_sync.go` soft-parse-failure log site emits `parse_error_count` and `first_error_line` only; the `first_error_message` field was removed because `textdic.Process` errors flow through a goyacc `Error(string)` path that could be customised to echo tokens.
