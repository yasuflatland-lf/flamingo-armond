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

The trinary is single-sourced in one field-agnostic value object, `trinaryText`,
that carries a private `*string`. Its parser mirrors `ParseCardText`: the caller
supplies the grapheme cap and the too-long sentinel, so the VO stays field-agnostic
(see [`caller-supplied-sentinels-in-parser.md`](caller-supplied-sentinels-in-parser.md)).

```go
// domain/trinary_text.go

type trinaryText struct {
    value *string
}

func parseTrinaryText(s *string, max int, tooLongErr error) (trinaryText, error) {
    if tooLongErr == nil {
        panic("domain: parseTrinaryText requires a non-nil tooLongErr sentinel")
    }
    if s == nil {
        return trinaryText{}, nil        // no change
    }
    trimmed := strings.TrimSpace(*s)
    if uniseg.GraphemeClusterCount(trimmed) > max {
        return trinaryText{}, tooLongErr
    }
    return trinaryText{value: &trimmed}, nil
}

func (t trinaryText) IsSet() bool { return t.value != nil }

func (t trinaryText) Ptr() *string {
    if t.value == nil {
        return nil
    }
    s := *t.value
    return &s    // copy to prevent aliasing mutation
}
```

`Bio` and `Description` are distinct exported types that embed `trinaryText`, so
`IsSet()`/`Ptr()` and the trim + grapheme-cap + trinary rule are shared. Each field
keeps its own cap constant and too-long sentinel; the `Parse*` and `*FromPtr` entry
points stay stable so consumers do not churn:

```go
// domain/bio.go

type Bio struct{ trinaryText }

func ParseBio(s *string) (Bio, error) {
    t, err := parseTrinaryText(s, BioMax, ErrBioTooLong)
    return Bio{t}, err
}

func BioFromPtr(p *string) Bio { return Bio{trinaryTextFromPtr(p)} }

// domain/description.go — Description embeds the same trinaryText, passing
// DescriptionMax / ErrDescriptionTooLong to the same parseTrinaryText body.
```

- `ParseBio(nil)` → `Bio{}` (no change): `IsSet()` returns `false`.
- `ParseBio(&"")` or `ParseBio(&"   ")` → explicit clear: `IsSet()` returns `true`,
  `Ptr()` returns a pointer to `""`. Whitespace-only inputs are collapsed to the
  explicit-clear case after trimming.
- `ParseBio(&"hello")` → set: `IsSet()` returns `true`, `Ptr()` returns a pointer
  to `"hello"`.

`Ptr()` returns a copy of the pointer's target so external mutation of the
returned pointer does not alter the value object's internal state.

## Usecase guard

The `if in.Bio != nil` check in `usecase/user.go` and `usecase/admin_user.go` mirrors
the `ParseBio(nil)` no-change case in Go guard-clause form. It is a defense against
future semantic changes to `ParseBio` (e.g. a version that validates a nil input
differently) rather than a correction for a current bug. Both the guard and the `nil`
pass-through inside `ParseBio` produce the same outcome today.
