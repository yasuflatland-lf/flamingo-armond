# Generic helper vs aggregate-specific wrap prefix

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.
> Related: [Canonical layer prefix](../../../.claude/rules/error-wrapping.md#canonical-layer-prefix).

## Why

The canonical-layer-prefix rule states that the `layer:` token is the package
name. Inside `internal/domain/` most wraps originate inside a named aggregate
constructor or method, so a second segment naturally identifies the aggregate:

```
domain: swipe record: new uuid v7
domain: card: parse front
```

When a reader sees `domain: swipe record:` in a log chain they know to look in
the swipe-record aggregate code. The prefix is a grep anchor.

`domain.NewID()` is different. It is an aggregate-agnostic generic helper — any
aggregate constructor can call it. Wrapping with an aggregate prefix would claim
the error originated inside that aggregate, which is false. Every aggregate that
calls `domain.NewID()` would need a separate identical handler, or one of them
would claim ownership it does not have.

## What

When a helper is aggregate-agnostic (callable from any aggregate in the package),
use the bare package name as the wrap prefix:

```go
// domain/id.go

// NewID returns a new UUIDv7 string for use as a domain entity ID.
// The zero value ("") is invalid; callers must treat a non-nil error as fatal.
func NewID() (string, error) {
    id, err := uuid.NewV7()
    if err != nil {
        return "", eris.Wrap(err, "domain: new uuid v7")
        //                       ^^^^^^ bare package name, not an aggregate
    }
    return id.String(), nil
}
```

When a helper is aggregate-specific (called from exactly one aggregate's
constructor or method), use the aggregate name as the second segment:

```go
// domain/swipe_record.go

// NewSwipeRecord constructs a SwipeRecord from the given input.
func NewSwipeRecord(cardID, userID string) (*SwipeRecord, error) {
    id, err := NewID()
    if err != nil {
        // NewID already wraps with "domain:"; do NOT re-wrap here — that
        // produces a double-wrap. Call NewID and propagate the error as-is.
        return nil, err
    }
    // ...aggregate-specific validation wraps use the aggregate prefix:
    if cardID == "" {
        return nil, eris.New("domain: swipe record: card id required")
    }
    return &SwipeRecord{ID: id, CardID: cardID, UserID: userID}, nil
}
```

## Decision rule

| Helper type | Example | Wrap prefix |
|---|---|---|
| Aggregate-agnostic generic | `domain.NewID()` | `domain:` (bare package) |
| Aggregate-specific | code inside `NewSwipeRecord`, `NewCard`, etc. | `domain: <aggregate>:` |

The test: if removing the call from aggregate A and adding it to aggregate B
would require renaming the wrap prefix, it is aggregate-specific. If the prefix
would stay the same, it is generic.

## Why the distinction matters

Error chain entries are grep anchors. A future contributor seeing
`domain: swipe record: new uuid v7` in a log will search
`internal/domain/swipe_record.go`. If `domain.NewID()` were wrapped with
`domain: swipe record:`, that search would point at the wrong file — the
wrapping happens inside `id.go`, not `swipe_record.go`.

The same grep discipline that motivates the two-segment prefix for usecase files
(`usecase: card: find by id`) motivates the package-level prefix for generic
domain helpers: the prefix should name the producing site, not the consuming site.
