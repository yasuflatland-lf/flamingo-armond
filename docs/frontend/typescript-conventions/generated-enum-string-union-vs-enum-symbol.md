# Consume the string-union enum from `@/generated/graphql`, not the runtime `enum` from `@/generated/base-types`

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

`graphql-codegen` emits **two** TypeScript symbols for every GraphQL enum, in two different files, and they are not interchangeable:

| Symbol | File | Shape | Generated for |
|---|---|---|---|
| `CefrLevel` | `@/generated/graphql` | string-union **type** (`'A1' \| 'A2' \| ...`) | every operation result / the whole data flow |
| `CefrLevel` | `@/generated/base-types` | runtime **`enum`** (`enum CefrLevel { A1 = 'A1', ... }`) | test fixtures that build mock payloads from the full schema shape |

The split is dictated by `frontend/codegen.ts`: the `client` preset emits operation/input/enum **types** under `src/generated/` (these are the union forms every query result uses), while the separate `base-types.ts` plugin emits the full schema's object types and runtime enums "for test fixtures that build mock payloads from the full schema shape."

## Assignability is one-directional

The two symbols are **not mutually assignable** (tsc-verified):

- string-union → enum **fails** (`Type '"A1"' is not assignable to type 'CefrLevel'` — the enum member is a branded value, not the bare string).
- enum → string-union **succeeds** (the enum members' string values are members of the union).

So a component that types its prop with the runtime enum forces every caller arriving from a query result (which carries the union) to insert an `as` cast plus the second import.

## What to do: type the prop with the string-union

A component consuming a generated enum should type its prop with the **string-union** from `@/generated/graphql` — the same symbol the query result and any hand-written data type (e.g. `SwipeCardData`) already use. This gives **one source of truth** for the level type across the query, the DTO, and the component prop, with no cast at the data → prop boundary.

```ts
import type { CefrLevel } from "@/generated/graphql"; // the string-union type

interface CefrBadgeProps {
  level: CefrLevel | null;
}
```

Keying an exhaustive `Record<CefrLevel, Band>` by string literals is **just as exhaustive** as keying it by the runtime enum — a future `C2` schema member is still a one-line compile error in the `Record`:

```ts
// Exhaustive over the union. Adding C2 to the schema breaks THIS line, not a default branch.
const bandOf: Record<CefrLevel, "a" | "b" | "c"> = {
  A1: "a", A2: "a", B1: "b", B2: "b", C1: "c",
};
```

This pairs with [required `string | null` over optional `?: string | null`](required-string-null-over-optional-string-null.md): `level: CefrLevel | null` is required-nullable, so a caller that forgets to wire the level fails to compile rather than silently passing `undefined`.

## Runtime-guard corollary: the union can lie at runtime

TypeScript erases the union at compile time; it cannot enforce union membership on a value that arrives over the wire. During a backend/frontend **deploy-skew window** a level outside the generated union (e.g. a new `C2` the backend already emits) can reach the component. Index access then returns `undefined` — made unconditionally explicit by `noUncheckedIndexedAccess` (enabled in this repo's `tsconfig`):

```ts
const band: "a" | "b" | "c" | undefined = bandOf[level];
if (band === undefined) {
  // Suppress an unstyled-but-present chip and surface the gap to developers,
  // rather than silently rendering a colorless badge.
  console.warn(`[CefrBadge] Unrecognized CEFR level "${level}" — badge suppressed.`);
  return null;
}
```

Keep `bandOf` typed as the exhaustive `Record<CefrLevel, ...>` so the **compile-time** exhaustiveness guarantee survives — the runtime guard is defense-in-depth for the skew window, not a replacement for it.

The same loud-failure posture as [auditing collapsed helpers for branches that lose all side effects](helper-inline-refactor-no-op-branches.md): the unknown case is a `console.warn`, never a silent fall-through.

## The `CefrLevel` casing origin

The `CEFRLevel` (schema/Go) → `CefrLevel` (codegen TS) casing comes from the same acronym-handling that the backend documents — see [`docs/backend/library-gotchas/gqlgen-acronym-enum-type-vs-method-casing.md`](../../backend/library-gotchas/gqlgen-acronym-enum-type-vs-method-casing.md). On the frontend, codegen lowercases the acronym tail of the field name (`cefrLevel`) into the emitted symbol name `CefrLevel`, so both generated symbols carry that single spelling.

Reference: `frontend/src/components/learn/cefr-badge.tsx` (typed on `@/generated/graphql`'s `CefrLevel`), `frontend/src/generated/graphql.ts` (union), `frontend/src/generated/base-types.ts` (enum), `frontend/codegen.ts` (the two `generates` entries).
