# Classifier check must run before any pipeline step that appends to the classified slice

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.

## What

When a function (a) classifies a slice of errors by inspecting every element, and (b) a subsequent pipeline step appends new errors to the same slice, the classification MUST happen before the append. Running the classifier after the append causes it to see elements it was not meant to classify, silently changing its verdict.

```go
// Good — classify first, then mutate.
if allDictionaryErrorsSkipped(parseErrs) {
    return SyncFromNotionOutput{ParseErrors: parseErrs}, nil
}
rows, parseErrs = dedupeParsedRows(rows, parseErrs)  // may append non-skip warnings

// Bad — dedupe runs first and adds "duplicate front" warnings (Kind: "DUPLICATE").
// allDictionaryErrorsSkipped then returns false for a genuinely skip-only payload.
rows, parseErrs = dedupeParsedRows(rows, parseErrs)
if allDictionaryErrorsSkipped(parseErrs) { ... }
```

Reference: `backend/internal/usecase/notion_sync.go` — `Sync` method, the skip-only short-circuit comment.

## Why this matters

`dedupeParsedRows` appends "duplicate front" validation entries (`Kind: "DUPLICATE"`) to `parseErrs` as a side effect of deduplication. If the skip-only classifier runs after `dedupeParsedRows`, a payload that contained only lone-front/lone-back lines (all `Kind: "FRONT_ONLY"` or `"BACK_ONLY"`) will also contain the duplicate warnings, making `allDictionaryErrorsSkipped` return `false` and falling through to the hard-failure branch — which deletes existing cards rather than preserving them.

## Generalisation

Any classifier-then-mutate pipeline shares this constraint:

1. **Read the slice** via the classifier.
2. **Only then** pass the slice to functions that can append, filter, or reorder it.

When the order constraint is non-obvious (because the classify call and the mutate call are far apart, or the mutate call has an innocuous name like "dedupe"), document the ordering invariant with a comment at the classify call site so future readers understand why the lines cannot be swapped.
