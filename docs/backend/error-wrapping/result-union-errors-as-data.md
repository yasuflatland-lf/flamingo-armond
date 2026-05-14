# Result Union: "errors as data" pattern

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.

The "errors as data" pattern surfaces routine business failures — outcomes the
frontend must branch on and render actionable UI for — as named members of a
GraphQL union rather than as top-level `gqlerror` entries. The live example is
`createCard`, which returns `CreateCardResult = CreateCardSuccess | CardDuplicateFrontError`.

## When to use Result Union vs. `BadUserInputWithExtensions`

| Situation | Preferred approach |
|---|---|
| Frontend needs typed, structured data to drive UX (e.g. "overwrite existing card?" dialog showing `existingBack`) | Result Union (`CreateCardResult`) |
| Plain field-validation failure where the frontend treats all cases uniformly (e.g. "front is required") | `BadUserInputWithExtensions` (see [`two-tier-api-pattern.md`](./two-tier-api-pattern.md)) |

The discriminator question: does the client need the variant's payload to make
a meaningful UX decision? If yes, encode the outcome in the schema — the codegen
produces typed variants the client can switch on safely.

## Schema-side conventions

Declare the error type with `implements UserError` (the existing schema interface
at `schema/schema.graphql:316`) so all business-error types share a `message:
String!` field:

```graphql
interface UserError {
  message: String!
}

type CardDuplicateFrontError implements UserError {
  message: String!
  existingCardId: ID!
  existingBack: String!
}

type CreateCardSuccess {
  card: Card!
}

union CreateCardResult = CreateCardSuccess | CardDuplicateFrontError
```

Adding a new variant to an existing union is non-breaking for clients that
handle unrecognised `__typename` values with a `default` branch. Removing a
variant is a breaking change.

## Backend — usecase layer

The usecase returns a **usecase-owned outcome type** and MUST NOT import
`backend/graph/model`. The layering rule: the graph package depends on the
usecase package, never the reverse.

```go
// CreateCardOutcome is the usecase-level result. Exactly one of Card or
// Duplicate is non-nil.
type CreateCardOutcome struct {
    Card      *domain.Card
    Duplicate *DuplicateCardInfo
}

// DuplicateCardInfo identifies the existing card that collided with the
// (cardgroup_id, front) unique index.
type DuplicateCardInfo struct {
    ExistingID   string
    ExistingBack string
}
```

The duplicate case is a typed value, not an `error`. Returning it as data
(rather than wrapping it in `gqlerr.BadUserInput`) is what allows the resolver
to map it to a union variant instead of a top-level error entry.

## Backend — resolver layer

The resolver maps `CreateCardOutcome` to the GraphQL union. The second return
value (`error`) is reserved for real failures — auth, internal, validation
errors that should NOT produce union variants:

```go
func (r *mutationResolver) CreateCard(ctx context.Context, input model.NewCardInput) (model.CreateCardResult, error) {
    outcome, err := r.CardUC.Create(ctx, usecase.CreateCardInput{...})
    if err != nil {
        return nil, err
    }
    if outcome.Duplicate != nil {
        return model.CardDuplicateFrontError{
            Message:        "A card with this front already exists in this cardgroup",
            ExistingCardID: outcome.Duplicate.ExistingID,
            ExistingBack:   outcome.Duplicate.ExistingBack,
        }, nil
    }
    if outcome.Card == nil {
        return nil, gqlerr.Internal(ctx, eris.New("resolver: CreateCardOutcome has no variant set"))
    }
    return model.CreateCardSuccess{Card: toCardModel(outcome.Card)}, nil
}
```

The exhaustiveness guard (`outcome.Card == nil`) converts an unset outcome into
an internal error rather than silently returning `nil, nil`, which GraphQL
serialises as a null result and confuses clients.

## Frontend — query and branching

The mutation selection set requests `__typename` plus per-variant inline fragments:

```graphql
mutation CreateCard($input: NewCardInput!) {
  createCard(input: $input) {
    __typename
    ... on CreateCardSuccess {
      card { id front back due state cardgroupId }
    }
    ... on CardDuplicateFrontError {
      message
      existingCardId
      existingBack
    }
  }
}
```

Branch on `__typename`, never on `extensions.code`:

```ts
const payload = result.data?.createCard;
if (payload?.__typename === "CardDuplicateFrontError") {
  // render "overwrite?" dialog using payload.existingCardId / payload.existingBack
  return;
}
// happy path: payload.__typename === "CreateCardSuccess"
```

The `extensions.code` / `extensions.reason` approach (described in
[`two-tier-api-pattern.md`](./two-tier-api-pattern.md)) is not needed when the
discriminator comes directly from the schema. Do not add extension-code parsing
helpers for cases already expressed as union variants — codegen provides the
type safety those helpers tried to recover by hand.

## Why union over extensions-code parsing

| Property | Result Union | Extensions-code parsing |
|---|---|---|
| Type safety | Codegen produces typed variants | Manual string constants, no compile check |
| Source of truth | Schema defines all outcomes | Schema and runtime diverge if a constant drifts |
| Additive changes | New variants are non-breaking; unhandled `__typename` falls through | New `reason` value can silently break matchers that miss it |
| Client coupling | Switches on `__typename` — no shared constant needed | Client and server must agree on the exact `reason` string |

Prefer Result Union when the variant set is small and schema-driven. Fall back
to `BadUserInputWithExtensions` when a full union type would be disproportionate
(e.g. a single optional extra field on an otherwise uniform validation error).
