# Typed classifier field over string-prefix matching at conversion boundaries

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.

## What

When a package-internal error type must be converted to a public type at a layer boundary, carry a **typed boolean field** alongside the human-readable message string. Callers branch on the field; the message string is UI-facing only.

```go
// textdic package (internal)
type parseError struct {
    Line    int
    Message string
    Skipped bool // true for grammar skip productions; false for lexer/parser failures
}

// textdic.ValidationError (public export)
type ValidationError struct {
    Line    int
    Message string
    Skipped bool // callers branch on this; never on Message prefix
}

// usecase/dictionary.go — conversion site
type DictionaryValidationError struct {
    Line    int    `json:"line"`
    Message string `json:"message"`
    Skipped bool   `json:"-"` // structural: never marshalled to wire; internal branching only
}
```

## Why

A classifier that matches `strings.HasPrefix(e.Message, "skipped:")` works until someone renames the prefix for a UI copy change, a localisation pass, or a grammar improvement. The rename looks harmless in the package that owns the string; it silently flips the classification in every caller that string-matched it. The bug only surfaces when a previously-skip-only sync starts deleting cards — a data-loss failure mode with no compiler signal.

A typed `Skipped bool` field:

- Cannot be broken by renaming the message string.
- Is explicit at the conversion site (`Skipped: e.Skipped` in the mapping loop).
- Carries the semantic through as many wrapping layers as needed without each layer re-parsing the string.

## Where the pattern applies

Any conversion boundary where an internal type must carry a discriminated category to an outer type:

1. Package-internal `parseError` → public `textdic.ValidationError` (`backend/internal/textdic/service.go`).
2. `textdic.ValidationError` → `usecase.DictionaryValidationError` (`backend/internal/usecase/dictionary.go`, `notion_sync.go`).
3. Any future parser or validator whose callers need to distinguish "intentionally skipped" from "genuinely failed" without re-parsing the message.

## Tag the UI-facing field with `json:"-"` when the classifier is internal-only

`DictionaryValidationError.Skipped` is tagged `json:"-"` because the GraphQL layer must never expose the raw classifier to clients — the wire format only carries `line` and `message`. The field still flows through every internal function that needs to branch on it. The `json:"-"` tag documents the intent explicitly rather than relying on callers to remember not to forward it.
