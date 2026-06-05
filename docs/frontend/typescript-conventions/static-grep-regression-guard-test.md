# Static-grep regression-rule tests

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

## Why

A grep-based test enforces a **settled architectural decision** — a deprecated pattern stays deleted, or a required pattern stays present. Naming the file `*-contract.test.ts` is misleading: "contract" implies an API surface that can be renegotiated by updating both sides. A regression guard has no renegotiable surface; the rule is closed. Naming the file `*-rule.test.ts` or `*-regression-guard.test.ts` signals enforcement, not specification, and helps reviewers distinguish it from behavioral tests at a glance.

## What

A regression-rule test file contains:

- A **top-of-file comment block** stating: (a) what tokens are forbidden and why, (b) what tokens are required and why, (c) what the file deliberately does NOT test (behavioral invariants), and (d) pointers to companion behavioral test files.
- A `productionSites` array enumerating the source files that must satisfy the rule.
- An `it.each(productionSites)` driver that reads each file with `readFileSync` and applies the rule assertions.
- `expect(source).toContain(<required token>)` for patterns that must be present.
- `expect(source).not.toMatch(<forbidden pattern>)` for deprecated identifiers that must stay absent.

The assertions are intentionally structural, not behavioral. Behavioral invariants (correct cursor values, deduplication, error-halt gates) live in per-file companion test files, not here.

## Worked example

`frontend/src/app/pagination-ref-triplet-removal-rule.test.ts` — the canonical reference, reproduced in full:

```ts
/**
 * Regression-rule test: ref-triplet removal and useEffectEvent adoption.
 *
 * WHAT THIS FILE TESTS
 * --------------------
 * Static string checks on production source files that enforce two rules:
 *   1. `useEffectEvent` is present — each pagination call site must use the
 *      React 19.2 Effect Event pattern to read the latest cursor/page state
 *      inside an IntersectionObserver callback without capturing stale values.
 *   2. The deprecated `endCursorRef` / `hasNextPageRef` / `searchQueryRef`
 *      ref-triplet identifiers are absent — these were the pre-migration
 *      stale-value capture refs (replaced by useEffectEvent latest-value reads)
 *      and must not re-appear after the migration landed.
 *
 * NOTE: `fetchingRef` is INTENTIONALLY retained in production as the same-tick
 * in-flight mutex, per `.claude/rules/pagination.md`. This rule does not assert
 * anything about `fetchingRef`.
 *
 * WHAT THIS FILE DOES NOT TEST
 * ----------------------------
 * Behavioral invariants (e.g. that fetchMore is called with the correct
 * cursor, that overlapping observer fires are deduplicated, that the error
 * halt gate stops the IO loop) are NOT tested here. Those invariants live in
 * the per-file behavioral test companions:
 *   - src/app/cardgroups/[id]/cards/use-cards-connection.test.ts
 *   - src/app/cardgroups/cardgroups-client.test.tsx
 *   - src/app/admin/users/admin-users-client.test.tsx
 *
 * This file is intentionally a grep-style regression guard: if a future
 * refactor accidentally reintroduces the ref-triplet pattern or removes the
 * useEffectEvent call, this test fails immediately without requiring a full
 * behavioral test run.
 */

import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

const productionSites = [
  {
    name: "cardgroups listing",
    sourcePath: "src/app/cardgroups/cardgroups-client.tsx",
  },
  {
    name: "cardgroup cards connection hook",
    sourcePath: "src/app/cardgroups/[id]/cards/use-cards-connection.ts",
  },
  {
    name: "admin users listing",
    sourcePath: "src/app/admin/users/admin-users-client.tsx",
  },
];

describe("pagination ref-triplet removal regression rule", () => {
  it.each(
    productionSites,
  )("$name: uses useEffectEvent and does not contain deprecated endCursorRef/hasNextPageRef/searchQueryRef identifiers", ({
    sourcePath,
  }) => {
    const source = readFileSync(join(process.cwd(), sourcePath), "utf8");

    // Rule 1: useEffectEvent must be present — confirms the React 19.2
    // Effect Event pattern is in use for observer-owned latest-value reads.
    expect(source).toContain("useEffectEvent");

    // Rule 2: the old ref-triplet identifiers must not reappear — these were
    // the pre-migration stale-value capture refs, replaced by useEffectEvent
    // latest-value reads, and are now deleted.
    expect(source).not.toMatch(/\b(endCursorRef|hasNextPageRef|searchQueryRef)\b/);
  });
});
```

## Naming convention

- `*-rule.test.ts` (preferred): signals a regression guard enforcing a settled rule.
- `*-regression-guard.test.ts`: equivalent, slightly more verbose.
- AVOID `*-contract.test.ts`: misleading — "contract" implies a renegotiable API surface.

## Gotchas

- `readFileSync` throws a raw `ENOENT` if a production file is renamed or moved. The failing `it.each` entry name carries the site name; optionally wrap in `try/catch` to re-throw with an explicit "was this file renamed?" message.
- This test does NOT verify behavior. Pair it with companion `.test.tsx` / `.test.ts` files that exercise actual runtime invariants. The rule test is the rot-loud signal; the behavioral tests are the substantive check.
- Scope `productionSites` to genuine call sites of the pattern. Including unrelated files adds noise when they legitimately lack the required token.

## See also

- `frontend/src/app/pagination-ref-triplet-removal-rule.test.ts` — canonical example.
- [`.claude/rules/pagination.md`](../../../.claude/rules/pagination.md) — the rule this guard enforces (no ref-triplet identifiers; `fetchingRef` intentionally retained).
- [`useEffectEvent` replaces ref mirrors for Effect-owned callbacks](useeffectevent-replaces-ref-mirror.md) — the migration this guard locks in.

## Second worked example: negative-pattern enforcement via `Component.toString()`

`frontend/src/app/learn/[cardgroupId]/learn-client.test.tsx` — `"does not carry optimisticResponse in the handleSwipe mutation"` — applies the same technique to enforce the Apollo `optimisticResponse` rule (see [`.claude/rules/pagination.md`](../../../.claude/rules/pagination.md) § "Drop `optimisticResponse` for mutations that can fail with typed GraphQL errors").

The difference from the `readFileSync` form: when the component is a named export available in the test module, `Component.toString()` retrieves the source without a filesystem path. The region-slice pins to the mutation call site rather than the whole file, keeping the assertion tight:

```ts
it("does not carry optimisticResponse in the handleSwipe mutation", () => {
  // handleSwipe can return InputValidationError (a typed GraphQL error variant).
  // Apollo v3 does not reliably roll back optimistic writes on typed errors —
  // only on network errors — so no `optimisticResponse` must appear in the call.
  const source = LearnClient.toString();

  const swipeStart = source.indexOf("handleSwipe({");
  expect(swipeStart).toBeGreaterThan(-1);

  const swipeCatchIdx = source.indexOf(".catch(", swipeStart);
  expect(swipeCatchIdx).toBeGreaterThan(-1);

  const swipeBlock = source.slice(swipeStart, swipeCatchIdx);
  expect(swipeBlock).not.toContain("optimisticResponse");
});
```

**Rationale.** This is negative-pattern enforcement at the source level — the same technique as the `endCursorRef` example, applied to a different concern. Where the ref-triplet guard asserts that a deprecated identifier stays absent after a migration, this guard asserts that a forbidden Apollo option never appears in a typed-error-capable mutation call. Both tests are structural, not behavioral: they fail on reintroduction without requiring any runtime mock setup. The `Component.toString()` variant avoids a hard-coded file path and works as long as the component is a module-level named export accessible in the test file.

## Third worked example: forbidden-capability set via `readFileSync` (module read-only by construction)

When the invariant is not "this one token stays absent" but "this module must **never acquire a whole capability**", grep the module source for the full set of tokens that capability would introduce. The capability is proven absent by construction: the import-statement absence is the compile-level half (the module cannot call what it never imports), and the source grep is the regression half (a future edit cannot smuggle the capability back in without tripping the test).

Worked example: `frontend/src/app/learn/[cardgroupId]/practice-client.test.tsx` — the `"PracticeClient FSRS-safe source guards"` block. Practice mode must be FSRS-safe: it re-arranges a purely-local queue and must NEVER write through the swipe mutation, because a write would corrupt the card's review schedule. The guard reads the module source and asserts the whole swipe-mutation surface is absent:

```ts
import { readFileSync } from "node:fs";
import { join } from "node:path";

const source = readFileSync(
  join(process.cwd(), "src/app/learn/[cardgroupId]/practice-client.tsx"),
  "utf8",
);

it("never references the HandleSwipe mutation", () => {
  expect(source).not.toContain("HandleSwipeMutation");
});
it("never imports or calls useMutation", () => {
  expect(source).not.toContain("useMutation");
});
it("never declares an optimisticResponse", () => {
  expect(source).not.toContain("optimisticResponse");
});
```

**Why `readFileSync`, not `Component.toString()`, for a forbidden-capability set.** The capability arrives through `import` statements (`import { useMutation } from "@apollo/client/react"`, the swipe-mutation document import). Imports are module-scoped — they live outside the component function body — so `Component.toString()` cannot see them and a `.toString()`-based grep would pass even if the module imported `useMutation`. Reading the source file is the only form that catches an import-level reintroduction. The three tokens (`HandleSwipeMutation`, `useMutation`, `optimisticResponse`) together cover the document, the hook, and the optimistic-write option; expand the set to any new token that would represent a path back to the forbidden capability.
