# Log a structured event when a batch item fails and earlier work will be dropped

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.

## What

When a loop processes a sequence of items and returns early on the first hard failure, the items processed before the failure are silently discarded. Log a structured error event at the failure site — not only at the caller — so operators can identify which item caused the abort without relying on the outer caller's generic error message.

```go
for i, page := range pages {
    words, parseErrs, err := textdic.Process(page.Text)
    if err != nil {
        // Earlier pages' rows are dropped when this function returns nil.
        // Log here so operators know which page caused the abort.
        if logger != nil {
            logger.ErrorContext(ctx, "notion sync: page parse failed",
                "page_index", i,
                "page_id",    page.ID,
                "error_name", reflect.TypeOf(err).String(),
            )
        }
        return nil, nil, err
    }
    // accumulate words and parseErrs ...
}
```

Reference: `backend/internal/usecase/notion_sync.go` — `parseNotionPages` function.

## Why

The caller (`Sync`) maps the error into `ErrNotionSyncParse` and returns an HTTP 422. That log line tells the operator the sync failed; it does not tell them *which* Notion page caused the failure or how many pages had already been parsed successfully before the abort. Without the inner log:

- An operator sees "notion sync: parse pages failed" in the outer log.
- They must re-run the sync with debug logging, or manually reproduce the page ordering, to find the bad page.
- Earlier successfully-parsed pages are already dropped, and those pages' absence from the sync output is invisible.

The inner log adds `page_index` and `page_id`, which are enough to retrieve the specific Notion page from the API and inspect the raw content.

## What to log (and what not to)

Log `page_index`, `page_id`, and `error_name` (the Go type of the error, e.g. `*errors.errorString`). Parser-derived snippets (`first_snippet`, `first_error_snippet`) are acceptable in logs because the input source is dictionary content, not personal data; there is no PII risk from the parser's token echoes.

## When the pattern applies

- Any loop that accumulates partial results and returns early on a hard error.
- The accumulated partial results are discarded (not returned alongside the error).
- The item that caused the failure is not identifiable from the outer caller's log line alone.

When the outer caller's log is already rich enough to identify the item (e.g. the loop processes exactly one item), the inner log is redundant and should be omitted.
