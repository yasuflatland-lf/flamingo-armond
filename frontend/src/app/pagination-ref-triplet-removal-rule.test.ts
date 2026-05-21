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
 *      ref-triplet identifiers are absent — these were the old same-tick mutex
 *      approach and must not re-appear after the migration landed.
 *
 * WHAT THIS FILE DOES NOT TEST
 * ----------------------------
 * Behavioral invariants (e.g. that fetchMore is called with the correct
 * cursor, that overlapping observer fires are deduplicated, that the error
 * halt gate stops the IO loop) are NOT tested here. Those invariants live in
 * the per-file behavioral test companions:
 *   - src/app/cardgroups/[id]/cards/use-cards-connection.test.ts
 *   - src/app/cardgroups/cardgroups-client.test.tsx
 *   - src/app/admin/users/AdminUsersClient.test.tsx
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
    sourcePath: "src/app/admin/users/AdminUsersClient.tsx",
  },
];

describe("pagination ref-triplet removal regression rule", () => {
  it.each(
    productionSites,
  )(
    "$name: uses useEffectEvent and does not contain deprecated endCursorRef/hasNextPageRef/searchQueryRef identifiers",
    ({ sourcePath }) => {
      const source = readFileSync(join(process.cwd(), sourcePath), "utf8");

      // Rule 1: useEffectEvent must be present — confirms the React 19.2
      // Effect Event pattern is in use for observer-owned latest-value reads.
      expect(source).toContain("useEffectEvent");

      // Rule 2: the old ref-triplet identifiers must not reappear — these were
      // the pre-migration same-tick mutex approach and are now deleted.
      expect(source).not.toMatch(/\b(endCursorRef|hasNextPageRef|searchQueryRef)\b/);
    },
  );
});
