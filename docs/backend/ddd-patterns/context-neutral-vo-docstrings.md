# Context-neutral docstrings on shared-context value objects

> Part of the [DDD patterns](./../../../.claude/rules/ddd-patterns.md) rules.

## Why

When the same value object encodes a trinary state that is consumed from
multiple contexts, a docstring that bakes in one context's meaning misleads
readers in the other. The `Bio` VO carries the same wire shape (`nil` /
pointer-to-`""` / pointer-to-non-empty) but the meaning diverges:

| Wire shape | Patch-context meaning (`UpdateProfileInput`) | Read-context meaning (DB column) |
|---|---|---|
| `nil` pointer | "leave the stored value unchanged" | "the column is NULL" |
| pointer to `""` | "explicitly clear the stored value" | "the column stores the empty string" |
| pointer to `"x"` | "set the stored value to `\"x\"`" | "the column stores `\"x\"`" |

A reader who finds `Bio` via the read path and sees a patch-context docstring
("`nil` means no change") will interpret a `BioFromPtr(nil)` result as "this
field was not part of the patch", which is wrong — the field is `NULL` in the
database. The reverse failure is just as bad: a patch-context caller who reads
"`nil` means NULL column" will not understand why omitting `Bio` from the input
preserves the existing value.

## What

Docstrings on the VO type, the constructors, and the accessors enumerate
**both** contexts. The aggregate or the consumer-specific helper carries the
context-specific intent.

### Bio (`backend/internal/domain/bio.go`)

```go
// Bio is the user profile bio, supporting a trinary: nil (no change),
// pointer-to-"" (explicit clear), or pointer-to-non-empty (set).
type Bio struct {
    value *string
}

// Ptr returns a fresh copy of the internal pointer:
//   - nil — the Bio holds no value (constructed via ParseBio(nil) or BioFromPtr(nil));
//   - non-nil pointer to empty string — the Bio holds an empty value;
//   - non-nil pointer to non-empty string — the Bio holds a populated value.
//
// Patch-context callers map nil → "no change", &"" → "explicit clear", &"x" → "set".
// Read-context callers map nil → NULL column, &"" → empty stored, &"x" → populated stored.
func (b Bio) Ptr() *string { ... }

// IsSet reports whether the Bio carries a value (its internal pointer is non-nil).
// Returns false for ParseBio(nil) (patch-context: no change) and for BioFromPtr(nil)
// (read-context: NULL column / zero value Bio{}). Returns true for any other
// construction, including an explicit empty string.
func (b Bio) IsSet() bool { return b.value != nil }
```

The accessor docstrings list the wire shape mapping (the "what") and then
enumerate the per-context interpretation (the "why this shape"). The reader
arriving from either context sees their own meaning without having to follow
the implementation trail to the call site.

### Context-specific helpers carry context-specific docstrings

`ParseBio` is patch-context: it takes user input and may surface
`ErrBioTooLong`. Its docstring talks about the patch contract.

`BioFromPtr` is read-context: it takes a database column value and never
errors. Its docstring talks about NULL columns and defensive copying.

```go
// ParseBio validates and trims s, preserving the trinary contract:
// nil means "no change"; a pointer to "" means "explicit clear"; a pointer to
// a non-empty string means "set". ...
func ParseBio(s *string) (Bio, error) { ... }

// BioFromPtr maps a nullable text column to a Bio: nil → Bio{} (IsSet()=false,
// NULL column); a non-nil pointer — including pointer-to-empty — maps to a set
// Bio whose Ptr() returns a defensive copy with the same string value.
// Used by repository readers to bridge a nullable text column into the typed
// domain field.
func BioFromPtr(p *string) Bio { ... }
```

The constructor that *receives* user input owns the validation contract; the
constructor that *receives* DB data trusts the DB and never validates. Two
constructors with one struct VO keeps the per-context concerns separated
while the VO itself stays single-source.

## The aggregate carries the context-shared posture

The `User` aggregate's field docstring touches all three contexts where `Bio`
appears (patch-context, read-context, and the in-memory aggregate state) and
explicitly disclaims the patch-context "no change" interpretation:

```go
// User is the application-owned row keyed by auth.users.id.
//
// DisplayName is *DisplayName so a NULL column round-trips as a nil pointer
// (no display name set). Bio is the trinary VO Bio (by value). The zero value
// Bio{} represents a NULL bio column (IsSet()=false); a set Bio carries either
// an explicit empty-string value or non-empty text. The trinary's "no change"
// meaning applies in the UpdateProfileInput patch context, not here.
type User struct {
    ID          string
    DisplayName *DisplayName
    Bio         Bio
    // ...
}
```

The "applies in the UpdateProfileInput patch context, not here" sentence is the
disambiguator: a reader who arrives via `User.Bio` is told that the trinary
they remember from the VO docstring is not the meaning in this read-context
field.

## When this matters

This pattern applies whenever a struct VO is shared by:

- An input DTO that uses a `nil` field to signal "absent".
- A repository row that uses a `nil` column to signal "NULL".

The trinary VO docstring chapter
([`docs/backend/ddd-patterns/trinary-value-object.md`](trinary-value-object.md))
covers the underlying VO design; this chapter covers the docstring discipline
that keeps the design legible from both ends.
