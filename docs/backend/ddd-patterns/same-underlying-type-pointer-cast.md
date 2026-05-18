# Same-underlying-type pointer cast for VO bridging

> Part of the [DDD patterns](./../../../.claude/rules/ddd-patterns.md) rules.

## Why

A repository row struct stores nullable string columns as `*string` (so that
NULL columns round-trip as a Go nil pointer). The domain field carrying the
same value is `*<DomainStringNewtype>` — the domain side wants the type
discipline of `DisplayName`, but the wire-shape contract (NULL → nil) is the
same as for the raw pointer.

The two types are not identical (one is `*string`, the other is `*domain.DisplayName`),
but their **underlying types** are identical (`*string`). Go's conversion
rules permit a direct pointer conversion between named pointer types whose
underlying types are identical, without an intermediate string copy and
without a nil check at the boundary:

```go
// backend/internal/repository/user.go — userToDomain
return &domain.User{
    ID:          g.ID,
    DisplayName: (*domain.DisplayName)(g.DisplayName),   // <- straight cast
    Bio:         domain.BioFromPtr(g.Bio),
    AvatarURL:   g.AvatarURL,
    // ...
}
```

The alternatives are strictly worse:

- **Helper function (`displayNameFromPtr`)**: introduces a function call, an
  extra symbol to find and read, and a nil-check that the cast does not need.
  Zero behavioural benefit because the conversion has no validation step.
- **Inline if-nil-else dereference**:
  ```go
  var dn *domain.DisplayName
  if g.DisplayName != nil {
      v := domain.DisplayName(*g.DisplayName)
      dn = &v
  }
  ```
  Five lines of stutter for the same outcome, and the `domain.DisplayName(*g.DisplayName)`
  step allocates a new string-header (technically harmless given immutable
  strings, but visually noisy).

The same cast applies in the other direction (`(*string)(domainPtr)`) if a
repository write path ever needs to thread the domain pointer back out
through a primitive field. Today the repository writes via a primitive-typed
`UserUpdate.DisplayName *string`, so the reverse cast does not appear.

## When the cast does NOT apply

The pointer cast is legal only when both sides have the **same underlying
type**. Two situations where it does not apply:

- **Struct VO (`Bio`)** — `Bio` is `struct { value *string }`, not a string
  newtype. There is no `(*Bio)(someStringPtr)` conversion; the bridging
  helper (`BioFromPtr`) is mandatory.
- **Underlying-type mismatch** — `type UserID uint64` cannot be cast from
  `*string` even if the on-the-wire shape is the same. The cast is a Go
  type-system operation, not a semantic equivalence.

## Reference

- `backend/internal/repository/user.go` — `userToDomain` uses the cast for `DisplayName` and falls back to `BioFromPtr` for `Bio`.
- `backend/internal/domain/display_name.go` — `type DisplayName string` (the underlying-type match that legalises the cast).
- `backend/internal/domain/bio.go` — `type Bio struct { value *string }` (the struct VO that requires the helper).
