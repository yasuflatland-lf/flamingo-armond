# XOR-invariant outcome structs for mutually-exclusive results

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

When a function has multiple mutually-exclusive successful outcomes — for example, "created
a record" versus "found an existing duplicate and returned its identity" — Go offers three
encoding options.

## Option 1: struct with `*X` and `*Y` exclusive pointers + documented XOR invariant

The simplest approach. Document the invariant in the struct comment, and add a runtime guard
at the consumer for belt-and-suspenders observability:

```go
// CreateCardOutcome is returned by CardUsecase.Create. Exactly one of Card or
// Duplicate is non-nil (XOR invariant). The duplicate-front case is surfaced as
// data, not as an error, so the resolver can map it to a GraphQL union variant.
type CreateCardOutcome struct {
    Card      *domain.Card      // non-nil on the happy path
    Duplicate *DuplicateCardInfo // non-nil when (cardgroup_id, front) is already taken
}
```

At the resolver — the single consumer — guard both impossible states explicitly:

```go
outcome, err := r.CardUC.Create(ctx, input)
if err != nil {
    return nil, err
}
if outcome.Duplicate != nil {
    return model.CardDuplicateFrontError{ /* ... */ }, nil
}
if outcome.Card == nil {
    // Both fields nil: XOR invariant violated. Convert to INTERNAL rather than
    // returning a schema null that silently violates the non-null return type.
    return nil, gqlerr.Internal(ctx,
        eris.New("resolver: CreateCardOutcome has no variant set"))
}
return model.CreateCardSuccess{Card: toCardModel(outcome.Card)}, nil
```

The "both non-nil" case is implicitly handled: `outcome.Duplicate != nil` wins and the
resolver returns the error variant, while the card is silently unused. For tighter
observability, add an explicit guard before the `Duplicate` check if both variants being
set signals a producer bug.

**Pros**: zero boilerplate, easy to read, Go-idiomatic for small structs.
**Cons**: the type does not enforce the invariant at compile time. A future producer that
sets both fields or neither will compile cleanly. The runtime guard at the resolver makes
this drift loud in tests and production, but only at the consumer, not at the producer.

## Option 2: sealed interface (sum type)

```go
type CreateCardResult interface{ isCreateCardResult() }
type CreateCardSuccess struct { Card *domain.Card }
type CardDuplicateFrontError struct { ExistingID, ExistingBack string }
func (CreateCardSuccess) isCreateCardResult()     {}
func (CardDuplicateFrontError) isCreateCardResult() {}
```

Every consumer must type-switch. The compiler enforces exhaustiveness only if the switch
includes a `default` panic.

**Pros**: compile-time enforcement that exactly one variant is returned (the producer
returns a concrete type, not a struct with pointer fields).
**Cons**: boilerplate for every consumer. In Go, sealed interfaces require a package-private
marker method, which leaks implementation detail.

**Migrate to this option when**: there are multiple producers, or the variant logic is
complex enough that the type system's enforcement is worth the boilerplate cost.

## Option 3: multiple return values

```go
func (u *CardUsecase) Create(ctx context.Context, in CreateCardInput) (
    card *domain.Card,
    dup  *DuplicateCardInfo,
    err  error,
) { ... }
```

Clear at the call site, but the result cannot be stored in a variable and passed around
without introducing an intermediate struct — which is option 1.

## Why this project uses option 1 for `CreateCardOutcome`

The current producer/consumer count is 1+1 (`CardUsecase.Create` + `mutationResolver.CreateCard`).
The runtime guard at the resolver converts the invariant violation into an INTERNAL error
that surfaces in tests and structured logs, making drift loud without the boilerplate cost
of a sealed interface. If a second producer or consumer is added, migrate to option 2.
