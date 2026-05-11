# Typed classifier field over string-prefix matching at conversion boundaries

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.

## What

When a package-internal error type must be converted to a public type at a layer boundary, carry a **typed enum field** alongside the human-readable message string. Callers branch on the field; the message string is UI-facing only. The enum's `String()` method must align with the wire format (JSON, GraphQL) so the same value flows through Go, JSON, and GraphQL without intermediate remapping.

```go
// textdic package (internal)
type SkipKind uint8

const (
    SkipKindUnknown      SkipKind = iota
    SkipKindHard
    SkipKindFrontOnly
    SkipKindBackOnly
    SkipKindUnrecognized
)

func (k SkipKind) String() string {
    switch k {
    case SkipKindHard:       return "HARD"
    case SkipKindFrontOnly:  return "FRONT_ONLY"
    case SkipKindBackOnly:   return "BACK_ONLY"
    case SkipKindUnrecognized: return "UNRECOGNIZED"
    default:                 return "UNKNOWN"
    }
}

// usecase/dictionary.go — conversion site
type DictionaryValidationError struct {
    Line    int       `json:"line"`
    Message string    `json:"message"`
    Kind    SkipKind  `json:"kind"` // serialized via String(); callers branch on the enum value
}
```

## Why

A classifier that matches `strings.HasPrefix(e.Message, "skipped:")` works until someone renames the prefix for a UI copy change, a localisation pass, or a grammar improvement. The rename looks harmless in the package that owns the string; it silently flips the classification in every caller that string-matched it. The bug only surfaces when a previously-skip-only sync starts deleting cards — a data-loss failure mode with no compiler signal.

A typed enum field with a wire-aligned `String()` method:

- Cannot be broken by renaming the message string.
- Is explicit at the conversion site (`Kind: e.Kind` in the mapping loop).
- Marshals directly to the wire (JSON, GraphQL) via `String()`, eliminating a category of remapping bugs.
- Carries the semantic through as many wrapping layers as needed without each layer re-parsing the string.

## Where the pattern applies

Any conversion boundary where an internal type must carry a discriminated category to an outer type:

1. Package-internal `parseError` → public `textdic.ValidationError` (`backend/internal/textdic/service.go`).
2. `textdic.ValidationError` → `usecase.DictionaryValidationError` (`backend/internal/usecase/dictionary.go`, `notion_sync.go`).
3. Any future parser or validator whose callers need to distinguish multiple failure modes without re-parsing the message.
