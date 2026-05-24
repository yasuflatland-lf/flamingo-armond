# Derive `optimisticResponse` types from generated mutation types — never duplicate inline

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.
> Cross-references [`docs/backend/error-wrapping/result-union-errors-as-data.md`](../../backend/error-wrapping/result-union-errors-as-data.md).

## Why

Apollo's `optimisticResponse` option requires the caller to construct a
synthetic mutation payload that matches the generated GraphQL return type
exactly — every `__typename`, every nested field, every union variant
shape. A typo or a missing field surfaces only when the type-checker runs,
and even a passing type-check does not catch a schema drift the inline
type itself was wrong about.

The temptation is to declare an inline `type X = { ... }` next to the
`optimisticResponse` literal, copying the schema shape by hand. Two failure
modes:

1. **Schema-additive drift rots silently.** A backend that adds a new
   non-null scalar to `PerformanceMetrics` regenerates the frontend's
   `HandleSwipeMutation` type, and TypeScript flags the inline duplicate
   as incomplete — but only if the inline type happens to be assigned
   back to the generated shape somewhere in the call site. A naked
   inline type that flows nowhere through the typechecker (e.g. assigned
   only to a `DEFAULT_METRICS` constant) compiles forever against a
   stale shape; the `optimisticResponse` write is silently incomplete.

2. **Union-variant promotion changes the path.** When a mutation is
   promoted from a bare payload (`HandleSwipe { performanceMode, metrics }`)
   to a union (`HandleSwipe { ... on HandleSwipeSuccess { response { performanceMode, metrics } } }`),
   the field that used to live one hop from the mutation root now lives
   two hops deep. An inline duplicate captures the old shape; the
   promoted shape requires a manual rewrite of every reference to the
   inline type. A derived `Extract<...>` chain refactors automatically
   on the next regen.

## What

Derive the inline type from the generated mutation type using
`Extract<Union, { __typename: "Variant" }>` plus indexed access:

```ts
import type {
  HandleSwipeMutation as HandleSwipeMutationType,
} from "@/generated/graphql";

// Derived from the generated HandleSwipeMutationType so schema changes stay in sync automatically.
type PerformanceMetrics = Extract<
  HandleSwipeMutationType["handleSwipe"],
  { __typename: "HandleSwipeSuccess" }
>["response"]["metrics"];

const DEFAULT_METRICS: PerformanceMetrics = {
  __typename: "PerformanceMetrics",
  successRate: 0.5,
  // ... every field the schema declares; missing fields are a compile error.
};

handleSwipe({
  variables: { input },
  optimisticResponse: {
    __typename: "Mutation",
    handleSwipe: {
      __typename: "HandleSwipeSuccess" as const,
      response: {
        __typename: "SwipeResponse" as const,
        performanceMode: 1,
        metrics: DEFAULT_METRICS,
      },
    },
  },
});
```

The `Extract<Union, { __typename: "Variant" }>` step narrows the union
member; the trailing indexed-access chain walks down the synthetic
payload's shape. Both pieces follow the schema automatically.

The `as const` annotations on the `__typename` strings are required: the
literal type `"HandleSwipeSuccess"` is what discriminates the union; a
bare string widens to `string` and the optimistic write loses type
safety against the union members.

## How to apply

For any client component that constructs an `optimisticResponse` value:

1. Import the generated mutation type from `@/generated/graphql` under a
   short alias (`HandleSwipeMutation as HandleSwipeMutationType`).
2. Derive the inline scaffolding type via `Extract<Mutation["field"], { __typename: "SuccessVariant" }>`
   followed by `["nestedField"]["subfield"]` indexed access until the
   wanted shape is reached.
3. Pin the result to a `const` (e.g. `DEFAULT_METRICS`) so missing fields
   are a compile error at the literal site, not at the
   `optimisticResponse` assignment.
4. Add `as const` to every `__typename` string literal inside the
   `optimisticResponse` object so the union discriminators retain their
   literal types.

Avoid:

- Inline `type X = { __typename: "..."; field: number; ... }` declarations
  that duplicate the schema shape by hand.
- `optimisticResponse: { ... } as unknown as HandleSwipeMutation` casts —
  the cast hides every field-shape regression the type-checker would
  otherwise catch.

Reference:
`frontend/src/app/learn/[cardgroupId]/learn-client.tsx` —
the `PerformanceMetrics` type derived from `HandleSwipeMutationType`
and the `DEFAULT_METRICS` constant feeding the `handleSwipe`
optimistic write inside `onSwipe`.

## Why not skip `optimisticResponse` entirely

For mutations that can return typed `UserError` variants (e.g.
`InputValidationError`), `@apollo/client` v3.x does not consistently roll
back optimistic writes — see the rule in
[`.claude/rules/pagination.md` § "Drop `optimisticResponse` for mutations
that can fail with typed GraphQL errors"](../../../.claude/rules/pagination.md).
The promotion of `handleSwipe` to an outcome union makes that question
live: the safer default is to drop the optimistic write and pay one
round-trip of latency. When latency is measurable (the swipe queue is
the canonical case — the user expects the next card to appear without
waiting), the derived-type pattern above is the correct shape for the
optimistic payload; otherwise prefer the no-`optimisticResponse` form
and avoid the derivation entirely.
