# Application vs Presentation error responsibility split

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.
> Related: [`FromUsecaseError` — single conversion site](from-usecase-error-conversion-site.md).

## Why

The usecase layer produces a typed `*ucerr.ValidationError` regardless of which
Presentation-layer consumer eventually handles the error. The wire shape that
consumers expose to their callers is a **Presentation layer choice**, not an
Application layer concern. The usecase must not shape itself around any specific
consumer.

Two consumers of the same `*ucerr.ValidationError` may expose entirely different
wire shapes:

- A GraphQL resolver calls `gqlerr.FromUsecaseError`, which extracts `Field` and
  `Message` into a `BAD_USER_INPUT` error with structured `extensions.field` and
  `extensions.message` — the rich shape the frontend needs to render field-scoped
  hints.
- A REST handler (`internal/handler/notionsync/handler.go`) may surface the same
  error as a generic HTTP 400 body for API contract simplicity. The structured
  detail is not missing; it lives in the `error_chain` log attribute written by
  `LogWarn`, where operators can triage it.

Neither handler is "more correct" than the other — they make different API
contract choices for different clients. The usecase returns the same typed
error in both cases.

## Pattern

The usecase produces typed errors and chains sentinels with `errors.Join`:

```go
// usecase/swipe.go

// validate returns nil or a typed error describing the first constraint violation.
func (u *SwipeUsecase) validate(input SomeInput) error {
    if input.Field == "" {
        // sentinel carries domain identity; ValidationError carries the wire detail
        return errors.Join(domain.ErrFieldRequired,
            translateFieldErr(ucerr.NewValidationError("field", "field is required")))
    }
    return nil
}
```

The `errors.Join` call preserves both the domain sentinel (for `errors.Is` branching
by callers that need to dispatch on identity) and the typed `*ucerr.ValidationError`
(for `gqlerr.FromUsecaseError` and any `errors.As` consumer).

The **GraphQL resolver** extracts the rich shape:

```go
// graph/resolver/schema.resolvers.go
return nil, gqlerr.FromUsecaseError(ctx, u.UC.DoSomething(ctx, input))
// → BAD_USER_INPUT with extensions.field + extensions.message
```

The **REST handler** chooses a generic response and logs the detail:

```go
// internal/handler/notionsync/handler.go
if err := u.uc.DoSomething(ctx, input); err != nil {
    logging.LogWarn(ctx, err, "notionsync: handler: do something")
    // structured detail in error_chain; wire body is intentionally generic
    return echo.NewHTTPError(http.StatusBadRequest, "invalid input")
}
```

A **CLI or analytics consumer** can extract the typed payload directly:

```go
var ve *ucerr.ValidationError
if errors.As(err, &ve) {
    fmt.Fprintf(os.Stderr, "field %q: %s\n", ve.Field, ve.Message)
}
```

## Design principles

1. **Usecase is consumer-agnostic.** The typed error contract (`*ucerr.ValidationError`,
   sentinels) is stable across all present and future Presentation-layer consumers.
   Adding a new REST endpoint, a gRPC handler, or a CLI command does not require
   changing the usecase's return type.

2. **Wire shape is a Presentation decision.** Each handler chooses independently
   how much of the typed detail to surface. A REST handler that returns a generic
   400 body is not a bug; it is a deliberate API contract choice. A 1-2 line `// WHY`
   comment at the `errors.Join` site is sufficient documentation.

3. **Structured detail is preserved for operators via logging.** Even when the
   wire shape is generic, `LogWarn` (or `LogError`) writes the full `error_chain`
   attribute — including `*ucerr.ValidationError.Field` and `.Message` — so
   operators can triage without a code change.

4. **`errors.Join` is the right tool when domain sentinel + wire detail coexist.**
   `errors.Join(domainSentinel, translateErr(ucerr.NewValidationError(...)))` keeps
   both the `errors.Is`-matchable sentinel and the `errors.As`-extractable typed
   value in the same chain. Neither is discarded.

## Anti-pattern: usecase returning a consumer-specific shape

A usecase that constructs a `*gqlerror.Error` (or any transport-specific type)
to satisfy a GraphQL contract couples business logic to the presentation protocol:

```go
// BAD: usecase returns a wire-format type.
func (u *SwipeUsecase) DoSomething(ctx context.Context, input SomeInput) error {
    // ...
    return gqlerr.BadUserInput("field", "field is required") // gqlerr import in usecase
}
```

This breaks the import-graph invariant enforced by `go-arch-lint`
([`backend-layering.md`](../../../.claude/rules/backend-layering.md)) and makes
the usecase untestable without the transport layer. REST handlers and CLI consumers
cannot interpret the wire-format error without importing the GraphQL transport package.
