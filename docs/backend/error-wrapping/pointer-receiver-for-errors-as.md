# Pointer-receiver discipline for typed errors used with `errors.As`

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.

## What

A typed error returned from a usecase or repository **must** define `Error() string` on a pointer receiver and be constructed via `&T{...}`. Callers that recover the value via `errors.As(err, &target)` must declare `target` as a pointer-to-struct (`var t *T`). Mixing value-receiver definitions with pointer-receiver `errors.As` targets — or vice versa — silently falls through to the default branch:

```go
// Definition: pointer receiver, pointer-allocated returns.
type ValidationError struct { Field, Message string }
func (e *ValidationError) Error() string { return "..." }

// Producer: returns a pointer, never a value.
return &ValidationError{Field: "front", Message: "required"}

// Conversion site: target is *T, not T.
var ve *ValidationError
if errors.As(err, &ve) { /* matches */ }

// Or, Go 1.26+:
if ve, ok := errors.AsType[*ValidationError](err); ok { /* matches */ }
```

## Why

`errors.As(err, target)` matches **assignability**, not type identity. A `T` (value) and a `*T` (pointer) are distinct types: `errors.As` against a `**T` target only walks the chain looking for a `*T` link, and a `T{}` value in the chain is invisible to it. The reverse is also true.

The failure mode is silent and transport-shaped: the conversion site (e.g. `gqlerr.FromUsecaseError`) falls through to the default branch (`Internal`) and a validation error becomes `INTERNAL` on the wire — no compile error, no panic, no log entry that screams "this was a typed error that I missed." The first signal is a frontend bug report.

The pointer convention is the easier half of the contract to enforce because:

- It interacts well with eris: `eris.Wrap(&T{...}, "...")` preserves the `*T` link so `errors.As` continues to match through the chain. A `T{}` value link survives `eris.Wrap` but loses identity through any layer that takes an interface and re-issues a wrapped value.
- It matches the convention used by `*pgconn.PgError`, `*gqlerror.Error`, `*url.Error`, and most other stdlib / third-party typed errors.

## How to apply

Document the pointer-allocation requirement **on the type definition**, not only at the conversion site:

```go
// ValidationError is the field-scoped, message-bearing validation failure
// returned from usecases. The pointer-receiver shape is required so callers
// can recover the value with errors.As(err, new(*ValidationError)) even after
// eris.Wrap. Always construct as &ValidationError{...}; never return a value.
type ValidationError struct { Field, Message string }
func (e *ValidationError) Error() string { /* ... */ }
```

Cross-reference: the conversion-site rules in [`from-usecase-error-conversion-site.md`](from-usecase-error-conversion-site.md) assume every typed usecase error already follows this discipline.

## Test the invariant explicitly

The pointer-receiver discipline is silent when broken, so a unit test on the conversion site is the only mechanism that catches a regression. The `from_usecase_test.go` table includes both bare and `eris.Wrap`-wrapped inputs for every typed error precisely so a value-receiver mistake cannot pass review:

```go
// Representative table entries — both forms must classify identically.
{name: "ValidationError",         err: &ucerr.ValidationError{...}},
{name: "eris-wrapped Validation", err: eris.Wrap(&ucerr.ValidationError{...}, "outer: ...")},
```

When introducing a new typed error, add the pair of cases as part of the same change.
