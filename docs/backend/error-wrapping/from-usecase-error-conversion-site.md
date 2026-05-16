# `FromUsecaseError` — single conversion site

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.

## Design intent

Resolvers translate usecase errors into gqlerrors at the return statement:

```go
return result, gqlerr.FromUsecaseError(ctx, uc.DoSomething(ctx, input))
```

A single conversion point keeps `gqlerr.*` out of the usecase layer entirely (enforced after [#158](https://github.com/yasuflatland-lf/flamingo-armond/issues/158) by a CI gate), centralises all `extensions.code` parity decisions, and lets reviewers reason about transport-shape choices in one file (`backend/internal/gqlerr/errors.go`) rather than hunting across every resolver.

Before this helper existed, resolvers wrapped errors ad-hoc: some called `gqlerr.Internal`, some called `gqlerr.BadUserInput`, and a few called `gqlerr.Unauthenticated` — each with a slightly different context-passing convention. The helper makes the convention testable and the omission detectable.

## Branch order

`FromUsecaseError` checks in this order:

1. `errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)` → `Cancelled(ctx, err)`
2. `errors.Is(err, usecase.ErrUnauthenticated)` → `Unauthenticated()`
3. `errors.As(err, &ve)` where `ve` is `*usecase.ValidationError` → `BadUserInput(ve.Field, ve.Message)`
4. `errors.As(err, &fe)` where `fe` is `*usecase.ForbiddenError` → `NewForbidden(fe.Message)`
5. default → `Internal(ctx, err)`

Cancel is checked first because a cancelled context is observed before any application-level guard runs — treating a deadline-exceeded request as `UNAUTHENTICATED` would be a misclassification. Authentication comes before validation because an unauthenticated caller should not receive field-level error detail.

## Usecase error type taxonomy

| Origin | Type | Converted by |
|---|---|---|
| Domain invariants | `domain.ErrCardgroupNameRequired`, `domain.ErrCardgroupNameTooLong`, etc. (sentinels) | `Internal` (no structural match above) |
| Repository miss | `repository.ErrNotFound` (sentinel) | `Internal` unless the usecase re-wraps it into a `ValidationError` |
| Repository conflict | `ErrCardDuplicateFront` etc. (sentinels) | Resolver maps directly when the error is part of an outcome union |
| Outcome union | `CardDuplicateFrontError` (Option B struct) | Resolver maps directly — **not** via `FromUsecaseError` (see below) |
| Field-scoped validation | `*usecase.ValidationError` | `BadUserInput` |
| Auth (identity missing) | `usecase.ErrUnauthenticated` (sentinel) | `Unauthenticated` |
| Authz (permission denied) | `*usecase.ForbiddenError` | `NewForbidden` |
| Infrastructure (DB, tx) | unwrapped `error` | `Internal` |

Domain-level sentinels that callers should surface as user-facing validation errors are re-wrapped into `*usecase.ValidationError` at the usecase layer before returning, so the conversion site sees the typed wrapper rather than a raw sentinel.

## Asserting in usecase tests

Usecase tests assert the typed value, not the gqlerror message:

```go
// Sentinel — use errors.Is
err := uc.DoSomething(ctx, input)
assert.True(t, errors.Is(err, usecase.ErrUnauthenticated))

// Structured type — use errors.As (works through eris.Wrap chains)
var ve *usecase.ValidationError
assert.True(t, errors.As(err, &ve))
assert.Equal(t, "front", ve.Field)
assert.Contains(t, ve.Message, "required")
```

`errors.As` traverses eris chain links, so wrapping at a layer boundary (`eris.Wrap(err, "usecase: ...")`) does not break the assertion. This decouples usecase tests from transport shape: they never import `gqlerr` or inspect `extensions.code`.

## Logger source (out-of-scope deferral)

`FromUsecaseError` inherits the logging behaviour of `gqlerr.Internal` and `gqlerr.Cancelled`, both of which currently call `slog.Default()`. Moving logger injection into the usecase layer (issue [#154](https://github.com/yasuflatland-lf/flamingo-armond/issues/154) audit item X-7) is intentionally out of scope here. When that work lands, the logger source changes inside the `gqlerr` package; the `FromUsecaseError` signature stays the same and all resolvers continue to call it unchanged.

## Relationship to outcome unions

`CreateCardOutcome` (the `CardDuplicateFrontError` variant) is not replaced by `FromUsecaseError`. The two patterns are complementary:

- `FromUsecaseError` handles the `error` return path — errors the resolver historically wrapped manually.
- Outcome unions handle business-state results the client must structurally branch on via `__typename`. The resolver maps the union member directly to the GraphQL result type; it does not pass union members through `FromUsecaseError`.

When a resolver returns a union type, only the `error` component of the Go return tuple goes through `FromUsecaseError`. A non-nil union member and a nil error are independent.

Cross-reference: [`docs/backend/error-wrapping/result-union-errors-as-data.md`](result-union-errors-as-data.md).

## CI gates

Two gates in `.github/workflows/backend.yml` enforce the conversion-site contract:

1. **`gqlerr` import inside `backend/internal/usecase/`** — currently a warning; becomes a build error after [#158](https://github.com/yasuflatland-lf/flamingo-armond/issues/158) lands.
2. **Hardcoded `extensions.code` literals outside `internal/gqlerr/`** — currently zero occurrences; any new literal fails the build immediately.

Gate 1 ensures the usecase layer stays transport-agnostic. Gate 2 ensures all `extensions.code` decisions flow through `gqlerr.*` helpers, making the full set of codes greppable in one package.
