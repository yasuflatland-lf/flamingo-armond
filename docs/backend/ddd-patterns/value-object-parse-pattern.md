# Value object Parse pattern

> Part of the [DDD patterns](./../../../.claude/rules/ddd-patterns.md) rules.

## Why

Validation failures in a domain VO are expected business outcomes, not exceptional
conditions. A constructor that panics (or silently truncates) hides the failure
from the caller. An explicit `error` return forces the caller to handle it at the
trust boundary — the point where raw user input meets the domain model.

## What

The canonical VO constructor signature is:

```go
Parse<Type>(s string) (<Type>, error)
```

Applied in `backend/internal/domain/` for `RoleName`, `CardText`, `DisplayName`,
and `Bio`. Each `Parse*` function:

1. Trims surrounding whitespace.
2. Validates a lower bound (non-empty after trim).
3. Validates an upper bound (byte count or grapheme cluster count, depending on
   whether the field is ASCII-restricted or multi-script).
4. Returns either the typed value or a domain-owned sentinel error.

### RoleName example (`domain/role_name.go`)

```go
const RoleNameMax = 50

func ParseRoleName(s string) (RoleName, error) {
    normalized := strings.ToLower(strings.TrimSpace(s))
    if normalized == "" {
        return "", ErrRoleNameRequired
    }
    if len(normalized) > RoleNameMax {
        return "", ErrRoleNameTooLong
    }
    if !roleNamePattern.MatchString(normalized) {
        return "", ErrRoleNameInvalid
    }
    return RoleName(normalized), nil
}
```

`RoleName` is ASCII-restricted (the pattern enforces `[a-z0-9_-]`), so byte
length equals grapheme count and `len()` is correct.

### DisplayName example (`domain/display_name.go`)

```go
const DisplayNameMax = 50

func ParseDisplayName(s string) (DisplayName, error) {
    trimmed := strings.TrimSpace(s)
    n := uniseg.GraphemeClusterCount(trimmed)
    if n < 1 {
        return "", ErrDisplayNameRequired
    }
    if n > DisplayNameMax {
        return "", ErrDisplayNameTooLong
    }
    return DisplayName(trimmed), nil
}
```

Multi-script display names use `uniseg.GraphemeClusterCount` because a single
visible character can be multiple UTF-8 bytes. Byte-counting would impose a
different effective cap depending on the script.

## Caller responsibility

Callers at the usecase layer translate domain sentinels into wire-typed errors via a
`translate<Type>Err` helper (see
[`docs/backend/ddd-patterns/exported-bound-constants-prevent-message-drift.md`](exported-bound-constants-prevent-message-drift.md)).
The domain layer owns the cap; the usecase layer owns the user-facing message.
