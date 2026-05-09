# What `error_chain` looks like

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.

For an error `eris.Wrap(eris.New("inner failure"), "outer context")`, `eris.ToJSON(err, true)` produces (abbreviated):

```json
{
  "root": {
    "message": "inner failure",
    "stack": [
      "main.run:/path/server/main.go:42",
      "..."
    ]
  },
  "wrap": [
    {
      "message": "outer context",
      "stack": "main.run:/path/server/main.go:51"
    }
  ]
}
```

This is emitted as the `error_chain` field in the JSON log line, alongside `level=ERROR` and the human-readable `msg`. Render and any downstream collector (Datadog / Sentry / Cloud Logging) ingest this JSON without further parsing.

**`root.stack` vs `wrap[].stack` shapes differ.** `root.stack` is a JSON array of `"Method:File:Line"` strings. Each entry in `wrap[]` has a `stack` field that is a **single** `"Method:File:Line"` string, not an array. Writing a log parser or assertion that treats both as arrays is a silent bug — only `root.stack` is iterable.
