# Cross-module constant references in test descriptions are silent-rot coupling

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

A test description that names a sibling module's constant by its identifier couples the test to that constant's exact name. A rename of the constant (or its replacement by a different mechanism — a `Set` lookup, a different regex name, a config value) leaves the test description misleading with no compile-time signal. Prefer module-relative wording that names the responsibility, not the symbol.

**Illustrative example** (hypothetical — not a real file in this repo, chosen to mirror the shape of the actual pattern):

```ts
// AVOID: rots the moment CARDGROUP_EDIT_RE is renamed or inlined in header-create-action.ts.
// frontend/src/components/nav/logo-drawer.test.ts
describe("returns null when path is blocked by CARDGROUP_EDIT_RE in header-create-action.ts", () => { ... });

// PREFER: names the routing responsibility, not the internal regex constant.
// frontend/src/components/nav/logo-drawer.test.ts
describe("returns null for paths that do not match a known create route", () => { ... });
```

**Why:** linters do not check English prose. A `grep` for the renamed constant will not find the stale test description; reviewers checking the test diff against the production diff will not flag a description that still reads naturally. The misalignment is invisible until a future reader is confused enough to investigate.

**How to apply:** when a test description must reference a sibling module's behaviour, name the **module's responsibility** (e.g. "header-create-action's routing guard") rather than the **constant's identifier** (e.g. `CARDGROUP_EDIT_RE`). The same rule extends to source comments that justify a piece of code by referencing a sibling module.
