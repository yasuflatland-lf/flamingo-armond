/**
 * Regression-rule test: ref-triplet removal / useEffectEvent adoption AND the
 * shared-hook extraction for cursor-paginated list screens.
 *
 * WHAT THIS FILE TESTS
 * --------------------
 * Static string checks on production source files that enforce two families of
 * rules:
 *
 *   IO-owning sites (the files that build an IntersectionObserver loop directly):
 *     1. `useEffectEvent` is present — the React 19.2 Effect Event pattern is
 *        used to read the latest cursor/page state inside the observer callback
 *        without capturing stale values.
 *     2. The deprecated `endCursorRef` / `hasNextPageRef` / `searchQueryRef`
 *        ref-triplet identifiers are absent — these were the pre-migration
 *        stale-value capture refs, replaced by useEffectEvent latest-value
 *        reads, and must not re-appear.
 *
 *   Migrated sites (the screens/hooks that consume the shared
 *   `useConnectionPagination` hook instead of inlining the loop):
 *     3. `useConnectionPagination` is present — the screen routes its IO through
 *        the single shared implementation.
 *     4. No inline IO loop remains — `new IntersectionObserver` and
 *        `useEffectEvent` are absent (they live in the shared hook now), and the
 *        ref-triplet is still absent.
 *
 * The shared `src/lib/pagination/use-connection-pagination.ts` hook is the
 * canonical owner of the observer loop; cards / cardgroups / catalog and the
 * admin masters / users listings all consume it. The hook is the only remaining
 * IO-owning site.
 *
 * NOTE: `fetchingRef` is INTENTIONALLY retained in the shared hook as the
 * same-tick in-flight mutex, per `.claude/rules/pagination.md`. This rule does
 * not assert anything about `fetchingRef`.
 *
 * WHAT THIS FILE DOES NOT TEST
 * ----------------------------
 * Behavioral invariants (correct cursor on fetchMore, overlapping-fire dedup,
 * error halt gate) are NOT tested here. Those live in the behavioral companions:
 *   - src/lib/pagination/use-connection-pagination.test.tsx
 *   - src/app/cardgroups/[id]/cards/use-cards-connection.test.ts
 *   - src/app/cardgroups/cardgroups-client.test.tsx
 *   - src/app/catalog/catalog-client.test.tsx
 *   - src/app/admin/masters/admin-masters-client.test.tsx
 *   - src/app/admin/users/admin-users-client.test.tsx
 *
 * This file is intentionally a grep-style regression guard: if a future refactor
 * reintroduces the ref-triplet, drops useEffectEvent from an IO owner, or
 * re-inlines an observer loop into a migrated screen, this test fails
 * immediately without requiring a full behavioral test run.
 */

import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

const ioOwningSites = [
  {
    name: "connection pagination hook",
    sourcePath: "src/lib/pagination/use-connection-pagination.ts",
  },
];

const migratedSites = [
  {
    name: "cardgroups listing",
    sourcePath: "src/app/cardgroups/cardgroups-client.tsx",
  },
  {
    name: "catalog gallery",
    sourcePath: "src/app/catalog/catalog-client.tsx",
  },
  {
    name: "entity cards connection factory",
    sourcePath: "src/lib/pagination/use-entity-cards-connection.ts",
  },
  {
    name: "admin masters listing",
    sourcePath: "src/app/admin/masters/admin-masters-client.tsx",
  },
  {
    name: "admin users listing",
    sourcePath: "src/app/admin/users/admin-users-client.tsx",
  },
  {
    name: "merge from catalog sheet",
    sourcePath: "src/components/cardgroups/merge-from-catalog-sheet.tsx",
  },
];

function readSource(sourcePath: string): string {
  return readFileSync(join(process.cwd(), sourcePath), "utf8");
}

describe("pagination ref-triplet removal regression rule", () => {
  it.each(ioOwningSites)(
    "$name: uses useEffectEvent and does not contain deprecated endCursorRef/hasNextPageRef/searchQueryRef identifiers",
    ({ sourcePath }) => {
      const source = readSource(sourcePath);

      // Rule 1: useEffectEvent must be present — confirms the React 19.2
      // Effect Event pattern is in use for observer-owned latest-value reads.
      expect(source).toContain("useEffectEvent");

      // Rule 2: the old ref-triplet identifiers must not reappear — these were
      // the pre-migration stale-value capture refs, replaced by useEffectEvent
      // latest-value reads, and are now deleted.
      expect(source).not.toMatch(/\b(endCursorRef|hasNextPageRef|searchQueryRef)\b/);
    },
  );
});

describe("pagination shared-hook extraction regression rule", () => {
  it.each(migratedSites)(
    "$name: consumes useConnectionPagination and carries no inline IntersectionObserver IO loop",
    ({ sourcePath }) => {
      const source = readSource(sourcePath);

      // Rule 3: the screen routes its pagination through the shared hook.
      expect(source).toContain("useConnectionPagination");

      // Rule 4: no inline IO loop remains — the observer construction and the
      // Effect Event live in the shared hook now, and the ref-triplet stays out.
      expect(source).not.toContain("new IntersectionObserver");
      expect(source).not.toContain("useEffectEvent");
      expect(source).not.toMatch(/\b(endCursorRef|hasNextPageRef|searchQueryRef)\b/);
    },
  );
});
