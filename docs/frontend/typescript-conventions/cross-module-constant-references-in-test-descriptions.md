# Cross-module constant references in test descriptions are silent-rot coupling

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

A test description that names a sibling module's constant by its identifier — e.g. `"shadowed by HIDDEN_PATH_RE in global-fab.tsx"` — couples the test to that constant's exact name. A rename of the constant (or its replacement by a different mechanism, e.g. a `Set` lookup or a different regex name) leaves the test description misleading with no compile-time signal. Prefer module-relative wording that names the responsibility, not the symbol:

```ts
// AVOID: rots the moment the constant is renamed.
describe("is shadowed externally by HIDDEN_PATH_RE in global-fab.tsx", () => { ... });

// PREFER: frontend/src/components/nav/fab-action.test.ts
describe("is shadowed externally by GlobalFAB's hidden-path guard for /cardgroups/new", () => { ... });
```

**Why:** linters do not check English prose. A `grep` for the renamed constant will not find the stale test description; reviewers checking the test diff against the production diff will not flag a description that still reads naturally. The misalignment is invisible until a future reader is confused enough to investigate.

**How to apply:** when a test description must reference a sibling module's behaviour, name the **module's responsibility** (e.g. "GlobalFAB's hidden-path guard") rather than the **constant's identifier** (e.g. `HIDDEN_PATH_RE`). The same rule extends to source comments that justify a piece of code by referencing a sibling module. Reference: `frontend/src/components/nav/fab-action.test.ts` and `frontend/src/components/nav/header-add-card-link.test.tsx` after a review finding that referring to `HIDDEN_PATH_RE` by name in test prose would rot the moment the constant was replaced.
