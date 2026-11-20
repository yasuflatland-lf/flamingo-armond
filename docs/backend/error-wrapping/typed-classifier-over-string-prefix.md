# Typed classifier field over string-prefix matching at conversion boundaries

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.

## What

When a package-internal error type must be converted to a public type at a layer boundary, carry a **typed enum field** alongside the human-readable message string. Callers branch on the field; the message string is UI-facing only. Map internal kinds explicitly to public kinds so an unmapped value fails at the boundary.

```go
// --- inner layer: textdic package (parser-internal) ---
// backend/internal/textdic/service.go

type SkipKind uint8

const (
    SkipKindUnknown      SkipKind = iota
    SkipKindHard
    SkipKindFrontOnly
    SkipKindBackOnly
    SkipKindUnrecognized
)

// String returns a readable value for parser diagnostics.
func (k SkipKind) String() string {
    switch k {
    case SkipKindHard:         return "HARD"
    case SkipKindFrontOnly:    return "FRONT_ONLY"
    case SkipKindBackOnly:     return "BACK_ONLY"
    case SkipKindUnrecognized: return "UNRECOGNIZED"
    default:                   return "UNKNOWN"
    }
}

// ValidationError is the parser-internal type; callers convert it to the
// public usecase type at the layer boundary.
type ValidationError struct {
    Line    int
    Message string
    Kind    SkipKind
    Snippet string
}

// --- outer layer: usecase package (application boundary) ---
// backend/internal/usecase/card_import.go

// CardImportErrorKind values are mapped from the inner SkipKind by an explicit
// switch (cardImportErrorKindFromSkipKind); an unmapped kind is an error at the boundary.
type CardImportErrorKind string

const (
    CardImportErrKindHard         CardImportErrorKind = "HARD"
    CardImportErrKindFrontOnly    CardImportErrorKind = "FRONT_ONLY"
    CardImportErrKindBackOnly     CardImportErrorKind = "BACK_ONLY"
    CardImportErrKindUnrecognized CardImportErrorKind = "UNRECOGNIZED"
    CardImportErrKindDuplicate    CardImportErrorKind = "DUPLICATE"
)

type CardImportError struct {
    Line    int                 `json:"line"`
    Message string              `json:"message"`
    Kind    CardImportErrorKind `json:"kind"` // wire-ready string; callers branch on the typed constant
    Snippet string              `json:"snippet,omitempty"`
}

// --- conversion site: card_import.go mapping loop ---
// backend/internal/usecase/card_import.go

for _, e := range parseErrs {
    kind, err := cardImportErrorKindFromSkipKind(e.Kind)
    if err != nil {
        return nil, err
    }
    mappedErrs = append(mappedErrs, CardImportError{
        Line:    e.Line,
        Message: e.Message,
        Kind:    kind,
        Snippet: e.Snippet,
    })
}
```

## Why

A classifier that matches `strings.HasPrefix(e.Message, "skipped:")` works until someone renames the prefix for a UI copy change, a localisation pass, or a grammar improvement. The rename looks harmless in the package that owns the string; it silently flips the classification in every caller that string-matched it. The bug only surfaces when a previously-skip-only sync starts deleting cards — a data-loss failure mode with no compiler signal.

A typed enum field with an explicit boundary mapping:

- Cannot be broken by renaming the message string.
- Makes every parser kind explicit at the conversion site.
- Rejects an unmapped parser kind before it reaches the resolver.
- Carries the semantic through as many wrapping layers as needed without each layer re-parsing the string.

## Where the pattern applies

Any conversion boundary where an internal type must carry a discriminated category to an outer type:

1. Package-internal `parseError` → public `textdic.ValidationError` (`backend/internal/textdic/service.go`).
2. `textdic.ValidationError` → `usecase.CardImportError` (`backend/internal/usecase/card_import.go`, `notion_sync.go`).
3. Any future parser or validator whose callers need to distinguish multiple failure modes without re-parsing the message.
