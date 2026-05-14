# `slog.NewJSONHandler` renders attrs as JSON keys, not `key=value` pairs

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

`slog.NewJSONHandler` renders structured attributes as JSON object keys; `slog.NewTextHandler` renders them as `key=value` pairs. The two shapes are not interchangeable in any artifact that quotes a log line verbatim — operator runbooks, grep instructions, alert rules, log-pipeline parsers, or test fixtures. A call site like:

```go
logger.Info("event happened", "count", 3)
```

emits `{"msg":"event happened","count":3}` under a JSON handler and `msg="event happened" count=3` under a text handler. A runbook that instructs an operator to `grep count=3` against a JSON-handler stream returns zero matches; the inverse fails symmetrically.

Before quoting a log line in any operator-facing doc, check which handler the relevant `main()` constructs and match that rendering. Quick-reproduce snippets and test stubs often default to the text handler even when production runs the JSON handler — re-render the line before copying it into a production runbook.

**Sister rule:** [Log output assertions: always read the buffer or the assertion is dead code](slog-buffer-assertions-must-be-checked.md) — the test-side counterpart for verifying what was actually logged.
