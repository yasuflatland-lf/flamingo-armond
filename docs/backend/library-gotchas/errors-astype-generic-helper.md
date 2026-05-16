# `errors.AsType[T error]` — generic narrowing helper (Go 1.26+)

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

## What

Go 1.26 added the generic helper

```go
func AsType[T error](err error) (T, bool)
```

to the standard `errors` package. It replaces the two-line `var t T; if errors.As(err, &t)` pattern with a single expression whose result is scoped to the `if` block:

```go
// Before (still correct, but verbose and leaks `ve` past the branch):
var ve *ucerr.ValidationError
if errors.As(err, &ve) {
    return BadUserInput(ve.Field, ve.Message)
}

// After (Go 1.26+):
if ve, ok := errors.AsType[*ucerr.ValidationError](err); ok {
    return BadUserInput(ve.Field, ve.Message)
}
```

This repo runs `golang 1.26.2` (see `backend/.tool-versions`), so the helper is available everywhere in `backend/`.

## Why

- **Scope hygiene.** `var ve *T` declares the binding in the enclosing function scope, so `ve` lives past the `if` even when the branch did not match. `AsType[T]` introduces the binding inside the `if` initializer, so a subsequent reference outside the matching branch fails to compile.
- **Reads as a single classification.** The `errors.As` form requires the reader to glance at the declaration and the call to figure out the target type. `AsType[*T]` puts the target in the function-name position where it is read first.
- **Same matching semantics as `errors.As`.** `AsType` is a thin wrapper — it walks the same chain, including eris-wrapped values, so adoption is mechanical and behavior-preserving.

## When NOT to use

- **Pre-Go-1.26 code paths.** Any module pinned below 1.26 (none in this repo today, but worth checking when copying snippets out) must keep the `errors.As` form.
- **Multiple downstream uses of the captured value.** When the matched value is consumed across several branches in the same function, the explicit `var ve *T` declaration gives every branch a single readable name without re-running `AsType` per branch.

## Adoption

Today's call sites are `backend/internal/gqlerr/from_usecase.go` and `backend/internal/usecase/swipe.go`. New conversion-site classifiers should default to the `AsType` form unless one of the "when not to use" conditions applies.
