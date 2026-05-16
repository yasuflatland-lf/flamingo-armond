# `FromUsecaseError` — single conversion site

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.

## Design intent

Resolvers translate usecase errors into gqlerrors at the return statement:

```go
return result, gqlerr.FromUsecaseError(ctx, uc.DoSomething(ctx, input))
```

A single conversion point keeps `gqlerr.*` out of the usecase layer entirely (enforced after [#158](https://github.com/yasuflatland-lf/flamingo-armond/issues/158) by a CI gate), centralises all `extensions.code` parity decisions, and lets reviewers reason about transport-shape choices in one file (`backend/internal/gqlerr/from_usecase.go`) rather than hunting across every resolver.

Before this helper existed, resolvers wrapped errors ad-hoc: some called `gqlerr.Internal`, some called `gqlerr.BadUserInput`, and a few called `gqlerr.Unauthenticated` — each with a slightly different context-passing convention. The helper makes the convention testable and the omission detectable.

`FromUsecaseError` does not accept variadic `slog.Attr` — callers needing to attach triage context (e.g. an entity ID) to the INTERNAL log line should call `gqlerr.Internal(ctx, err, slog.String("...", ...))` directly rather than going through this helper.

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
| Outcome union | `CardDuplicateFrontError` (union variant struct returned in CreateCardResult) | Resolver maps directly — **not** via `FromUsecaseError` (see below) |
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

## Asserting in resolver tests

Resolver tests live one layer above the conversion site: they exercise the full resolver call, including the `FromUsecaseError(ctx, ...)` wrap, so the assertion target is the wire-format `extensions.code` string. The two halves of a resolver test therefore use different error vocabularies:

- **Mock-side errors** (what the stubbed usecase returns) MUST be typed errors — `&ucerr.ForbiddenError{Message: "..."}`, `&ucerr.ValidationError{Field, Message}`, `ucerr.ErrUnauthenticated`. Returning a `*gqlerror.Error` directly from a mock bypasses the conversion site entirely and lands in the resolver as an already-wire-shaped error; `FromUsecaseError` does not recognise the `extensions.code` because the structural matches (`errors.As[*usecase.ValidationError]`, etc.) all fail, and the resolver classifies it as `INTERNAL`. Every `extensions.code` assertion downstream goes red.
- **Assertion-side references** to wire-format constants — `gqlerr.CodeForbidden`, `gqlerr.CodeBadUserInput` — stay imported in the resolver test file. They live on the assertion side of the call, not the mock side, and reading them as constants (`string(gqlerr.CodeForbidden)`) is the canonical way to keep the test's expected code string in sync with the wire constant.

The two imports (`backend/internal/gqlerr` for code constants, `backend/internal/usecase/ucerr` for mock construction) coexist in the same resolver test file by design — they cover the two ends of the resolver's translation step. A mechanical migration that strips `gqlerr` imports from resolver tests because "we removed gqlerr from usecase" breaks the assertion-side references; grep for `gqlerr.Code*` usage before removing the import.

## Logger source (out-of-scope deferral)

Logger DI inside the usecase layer is intentionally not addressed here. This helper inherits the `slog.Default()` behavior of `gqlerr.Internal` / `gqlerr.Cancelled` unchanged. When the usecase layer is later wired to accept an injected logger, the change lands inside the `gqlerr` package and the `FromUsecaseError` signature stays the same.

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

Note: Case 1's grep is line-oriented and the underlying `grep -rln '"backend/internal/gqlerr"'` pattern matches the import path string only — comment-text mentions of `gqlerr` do not trigger the warning.

Note: Case 2 is also line-oriented; a multi-line map literal (`"code":` on one line, the literal on the next) or a literal assembled via `fmt.Sprintf` would not match. The primary defense against bypass is code review, not the grep itself.
