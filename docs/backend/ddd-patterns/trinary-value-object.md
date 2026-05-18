# Trinary value object for optional profile fields

> Part of the [DDD patterns](./../../../.claude/rules/ddd-patterns.md) rules.

## Why

A profile update mutation may send three distinct intents for an optional field:

| Intent | Meaning |
|---|---|
| Field absent from input | Leave the stored value unchanged |
| Field present, value `""` | Explicitly clear the stored value |
| Field present, value `"x"` | Set the stored value to `"x"` |

A bare `*string` cannot distinguish "leave alone" (no pointer) from the Go zero
value for the pointee. Using a separate `bool` flag alongside the pointer drifts
independently — the flag can say "set" while the pointer says `nil`, or vice versa.

## What

`Bio` encodes the trinary as a struct with a private `*string`:

```go
// domain/bio.go

type Bio struct {
    value *string
}

func ParseBio(s *string) (Bio, error) {
    if s == nil {
        return Bio{}, nil        // no change
    }
    trimmed := strings.TrimSpace(*s)
    if uniseg.GraphemeClusterCount(trimmed) > BioMax {
        return Bio{}, ErrBioTooLong
    }
    return Bio{value: &trimmed}, nil
}

func (b Bio) IsSet() bool    { return b.value != nil }

func (b Bio) Value() *string {
    if b.value == nil {
        return nil
    }
    s := *b.value
    return &s    // copy to prevent aliasing mutation
}
```

- `ParseBio(nil)` → `Bio{}` (no change): `IsSet()` returns `false`.
- `ParseBio(&"")` or `ParseBio(&"   ")` → explicit clear: `IsSet()` returns `true`,
  `Value()` returns a pointer to `""`. Whitespace-only inputs are collapsed to the
  explicit-clear case after trimming.
- `ParseBio(&"hello")` → set: `IsSet()` returns `true`, `Value()` returns a pointer
  to `"hello"`.

`Value()` returns a copy of the pointer's target so external mutation of the
returned pointer does not alter the `Bio`'s internal state.

## Usecase guard

The `if in.Bio != nil` check in `usecase/user.go` and `usecase/admin_user.go` mirrors
the `ParseBio(nil)` no-change case in Go guard-clause form. It is a defense against
future semantic changes to `ParseBio` (e.g. a version that validates a nil input
differently) rather than a correction for a current bug. Both the guard and the `nil`
pass-through inside `ParseBio` produce the same outcome today.
