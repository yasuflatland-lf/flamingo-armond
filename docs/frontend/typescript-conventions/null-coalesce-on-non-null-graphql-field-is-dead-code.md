# `?? []` on a non-nullable GraphQL list field is dead code

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

When a GraphQL schema declares a field as `[T!]!` — a non-nullable list of non-null items — codegen emits the TypeScript type `T[]`, not `T[] | null | undefined`. The `??` operator short-circuits only when its left-hand operand is `null` or `undefined`, so `data.field ?? []` is unreachable: TypeScript accepts `T[]` as already guaranteed non-null, and the fallback branch never activates at runtime.

```ts
// schema/card.graphql
// learnNextDueCards(cardgroupId: ID!, limit: Int = 20): [Card!]!

// frontend/src/generated/graphql.ts (codegen output)
export type LearnNextDueCardsQuery = {
  __typename?: 'Query',
  learnNextDueCards: Array<{ __typename?: 'Card', id: string, ... }>
  //                 ^^^^^ — T[], not T[] | null | undefined
};
```

Writing `?? []` on such a field implies to every reader that the field might be absent, which is false. It is a small but real code smell: it erodes trust in the type-level contract and seeds a "what does null mean here?" question that has no answer.

```ts
// AVOID: ?? [] is unreachable; misleads readers into thinking the field can be null.
const cards = cardsData.learnNextDueCards ?? [];

// PREFER: read the field directly — the schema and codegen guarantee it is always an array.
const cards = cardsData.learnNextDueCards;
```

Reference: `frontend/src/app/learn/[cardgroupId]/page.tsx`, where the `?? []` guard was removed from `cardsData.learnNextDueCards` because the schema field is `[Card!]!` and codegen produces a plain `Array<Card>`.

### What actually surfaces field-level errors

A runtime failure on a `[T!]!` field does not produce `null` for that field in the client. The outcome depends on the response shape:

- **No `data`** — `gqlFetch` throws an `Error("GraphQL errors: " + ...)`, surfacing through the RSC `try/catch`. The `?? []` guard inside the `try` block is never reached.
- **Partial response** (`data != null` alongside `errors`) — `gqlFetch` returns `json.data` for non-auth errors, emitting a `console.warn`. If the error carries `UNAUTHENTICATED` or `FORBIDDEN`, `gqlFetch` re-throws rather than returning partial data.

Neither path produces a `null` value for the field after the `await` resolves without throwing. The `?? []` guard does not protect against either failure mode.

See [`docs/frontend/rsc-error-handling/partial-response-gqlfetch.md`](../rsc-error-handling/partial-response-gqlfetch.md) for the `gqlFetch` contract and [`docs/frontend/codegen.md`](../codegen.md) for how codegen types are generated and consumed.
