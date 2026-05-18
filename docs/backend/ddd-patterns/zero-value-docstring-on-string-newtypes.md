# Zero-value docstring on string newtypes

> Part of the [DDD patterns](./../../../.claude/rules/ddd-patterns.md) rules.

## Why

Go cannot seal a string newtype the way a struct type can be sealed via unexported
fields. Any caller can write `RoleName("")` or `CardText("")` and obtain the zero
value without going through `Parse*`. The zero value is always invalid — an empty
`RoleName` that bypasses `ParseRoleName` skips the length and pattern checks and
silently enters the system.

A docstring does not prevent the bypass, but it surfaces the gap:

- Code review sees the intent before approving a raw cast.
- IDE hover shows the warning before a developer writes the bypass.
- `go doc` and generated API references carry the constraint.

## What

Each string-newtype VO declares the zero-value invalidity on the type:

```go
// domain/role_name.go

// RoleName is the canonical, lowercase, slug-like identifier for an application
// role. ...
// The zero value (RoleName("")) is invalid; use ParseRoleName to construct.
type RoleName string
```

The same pattern appears in `domain/display_name.go`:

```go
// The zero value (DisplayName("")) is invalid; use ParseDisplayName to construct.
type DisplayName string
```

And in `domain/card_text.go`:

```go
// The zero value (CardText("")) is invalid; use ParseCardText to construct.
type CardText string
```

## Relationship to future compile-time enforcement

The docstring is a documentation pattern, not a runtime safeguard. Stronger
enforcement options — a struct wrapper with an unexported field, a linter rule that
forbids raw string conversion for these types — are tracked separately (see issue
#193). Until then, the docstring is the review-time and IDE-time checkpoint.
