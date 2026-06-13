# Re-export a generated GraphQL enum type instead of hand-mirroring it

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

## The problem

When a GraphQL schema defines an enum and a component layer needs to type a
prop or local variable with it, a hand-written mirror is tempting:

```ts
// frontend/src/components/learn/types.ts — ANTI-PATTERN
export type LearnDisplayMode = "FLIP_TO_REVEAL" | "ALWAYS_VISIBLE";
```

This compiles and passes type-checks — for now. The mirror is assignable to
the codegen-generated union by structural coincidence because the string
literals happen to match. Two silent failure modes follow:

1. **Schema extension rots the mirror.** When the schema adds a third value
   (e.g. `"ALWAYS_VISIBLE_REVERSED"`), codegen regenerates the generated
   union, but the hand mirror stays at two values. The new value fails the
   component's `=== "ALWAYS_VISIBLE"` exhaustiveness check silently — no
   compile error at the mirror, just a runtime misclassification far from the
   schema change.
2. **Structural coincidence is not enforced.** Nothing prevents the mirror
   from diverging (a typo, a rename) before anyone notices. The mirror looks
   like a local type; reviewers do not think to cross-check it against the
   schema.

## The fix: re-export the generated type

```ts
// frontend/src/components/learn/types.ts — CORRECT
export type { LearnDisplayMode } from "@/generated/graphql";
```

The generated module (`@/generated/graphql`) is not a route client; importing
from it respects the [type-placement rule](type-placement-in-component-trees.md)
that low-level UI primitives must not depend on route-level components.

Every consumer that previously imported from `types.ts` continues to work with
no import-path change:

```ts
// swipe-card-stack.tsx, learn-client.tsx, learn-client.test.tsx
import type { LearnDisplayMode } from "@/components/learn/types";
// LearnDisplayMode resolves to the codegen union — schema is the single source of truth.
```

The re-export also preserves the component layer's JSDoc docblock placement:
the schema-authored doc comment in `graphql.ts` travels with the type, and
the re-export site in `types.ts` can add a one-line context comment
(`// Re-export of the generated GraphQL LearnDisplayMode enum`) without
duplicating prose.

## When NOT to apply this rule

A type that has no schema counterpart must stay hand-written. The contrast case
in the same `types.ts` file:

```ts
// Reveal phase of the active card — local state, no schema counterpart.
export type LearnCardPhase = "front_only" | "revealed";
```

`LearnCardPhase` is pure client state: `"front_only"` and `"revealed"` never
appear in the GraphQL schema. Re-exporting from `@/generated/graphql` would
fail (the symbol does not exist there). The rule is: **re-export from
codegen when the enum originated in the schema; keep hand-written when the
union is entirely local**.

## Compile-time guarantee

Because the re-export is a `type` re-export pointing at the codegen output,
the TypeScript compiler tracks the full union from schema definition through
every call site. Adding a new enum member to the schema regenerates the
codegen file, which widens the re-exported union, which surfaces at every
exhaustive check (`Record<LearnDisplayMode, ...>`, `switch` with no default,
`===`-based narrowing) in one `tsc --noEmit` pass. The mirror approach
required a manual diffing step that was easy to skip; the re-export approach
makes the same discovery automatic.

Reference: `frontend/src/components/learn/types.ts` (the re-export),
`frontend/src/generated/graphql.ts` (the `LearnDisplayMode` string-union type
at line 63), `frontend/src/components/learn/swipe-card-stack.tsx` and
`frontend/src/app/learn/[cardgroupId]/learn-client.tsx` (consumers).

Related: [Consume the string-union enum from `@/generated/graphql`, not the
runtime `enum` from `@/generated/base-types`](generated-enum-string-union-vs-enum-symbol.md)
covers the two-symbol split (`graphql.ts` union vs. `base-types.ts` runtime
enum); this rule covers the hand-mirror vs. re-export choice upstream of that.
