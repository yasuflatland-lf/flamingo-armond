# Alias-bridge sub-package for cycle-safe shared types

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.

## What

When a callee package (here: `gqlerr`) needs to import shared types defined under a producer package (here: `usecase`), but the producer already imports the callee for legacy reasons, you can break the cycle by:

1. Putting the shared types in a **sub-package** of the producer (here: `usecase/ucerr/`).
2. Re-exporting them from the parent package via Go **type aliases** (`type T = ucerr.T`) and `var` re-bindings (`var Err = ucerr.Err`).

```go
// backend/internal/usecase/ucerr/errors.go
package ucerr

import "errors"

var ErrUnauthenticated = errors.New("usecase: not authenticated")

type ValidationError struct { Field, Message string }
func (e *ValidationError) Error() string { /* ... */ }

type ForbiddenError struct { Message string }
func (e *ForbiddenError) Error() string { /* ... */ }

// backend/internal/usecase/errors.go
package usecase

import "backend/internal/usecase/ucerr"

var ErrUnauthenticated = ucerr.ErrUnauthenticated
type ValidationError = ucerr.ValidationError
type ForbiddenError  = ucerr.ForbiddenError
```

`gqlerr.FromUsecaseError` then imports `backend/internal/usecase/ucerr` (no cycle), while resolvers and tests may write either `*usecase.ValidationError` or `*ucerr.ValidationError` interchangeably.

## Why

- **Direct import would cycle.** `usecase/*.go` (the legacy production code) still imports `gqlerr.*` for transport-shape decisions. A direct `gqlerr → usecase` import for the new error types closes a cycle the Go compiler rejects. The sub-package is the smallest cut that breaks it without forcing every legacy resolver to migrate first.
- **Aliases (`type T = U`), not new type definitions.** A new type definition (`type T U`) creates a distinct type — `errors.As(err, &usecase.ValidationError{})` would not match a `*ucerr.ValidationError` produced inside the usecase layer. The `=` form makes both names refer to the **same underlying type**, so `errors.Is` / `errors.As` succeed across both. `var` re-bindings have the same semantics for sentinels.
- **Migration optionality.** Callers (resolvers, tests, helpers) get to use the parent-package name (`usecase.ValidationError`) without any awareness that a sub-package exists. The split is purely an implementation detail of the producer side and can be collapsed later by moving the types back into `usecase/` once the legacy `gqlerr.*` imports are gone.

## Verification

The transparency invariant must be tested explicitly — it is easy to introduce a regression by changing `=` to `:=` (impossible at type level) or by switching a sentinel `var` to a constructor function.

```go
// backend/internal/gqlerr/from_usecase_test.go (representative)
func TestValidationErrorAliasIsTransparent(t *testing.T) {
    var ve *usecase.ValidationError
    require.True(t, errors.As(&ucerr.ValidationError{Field: "x"}, &ve))
}

func TestErrUnauthenticatedReexportIsTransparent(t *testing.T) {
    require.True(t, errors.Is(usecase.ErrUnauthenticated, ucerr.ErrUnauthenticated))
}
```

## When to reach for this pattern

Apply only when **both** conditions hold:

1. The producer package already imports the callee for unrelated, legacy reasons that are not being removed in the same change.
2. The shared types are small enough that splitting them into a sub-package costs less than the alternative (refactoring all the legacy callers in the same PR).

If the legacy imports can be removed in the same change, do that instead — the sub-package is permanent friction. Tracked migration: [#158](https://github.com/yasuflatland-lf/flamingo-armond/issues/158) collapses `usecase/ucerr` back into `usecase/` after the last legacy `gqlerr.*` import inside `usecase/` is gone.

## Verifying acyclicity at design time

Before adopting this layout, prove that the proposed import does not close a cycle by running the toolchain (`go build`, `go list`), not by reading the new types' import list. The plan author's import inventory will miss a cycle introduced by **legacy** imports in the target package. See [`.claude/rules/scope-discipline.md` § "Plan-document claims about the codebase's structure must be toolchain-verified"](../../../.claude/rules/scope-discipline.md#plan-document-claims-about-the-codebases-structure-must-be-toolchain-verified) for the general rule and a smoke-build recipe.
