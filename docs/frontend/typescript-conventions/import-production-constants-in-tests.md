# Import production constants in tests — never hardcode magic numbers for boundary conditions

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

When a test exercises a boundary condition defined by a production constant — a page-size limit, a prefetch threshold, a maximum retry count — import the constant from the production source and use it directly. Hardcoding a literal that mirrors the constant's current value creates silent drift: if the production constant changes, the test continues to pass at the old boundary and misses the regressions at the new one.

```ts
// WRONG — silent drift when LEARN_PAGE_LIMIT or PREFETCH_THRESHOLD changes.
it("fires a prefetch when queue length drops to 5 or below", async () => {
  const initial = makeQueue(5); // magic number
  const mock = buildMock({ variables: { cardgroupId: CG_ID, limit: 20 } }); // magic number
  ...
});

// CORRECT — test contract tracks the production boundary automatically.
import { LEARN_PAGE_LIMIT } from "@/app/learn/queries";
import { PREFETCH_THRESHOLD } from "./learn-client";

it("fires a prefetch when queue length drops to PREFETCH_THRESHOLD or below", async () => {
  const initial = makeQueue(PREFETCH_THRESHOLD);
  const mock = buildMock({ variables: { cardgroupId: CG_ID, limit: LEARN_PAGE_LIMIT } });
  ...
});
```

**Why this matters beyond correctness:** a test that uses a magic number encodes both the current value and the intent that "this value is the threshold" in the same literal. If the threshold moves from 5 to 8, the test silently covers the wrong range. With the import, any change to the constant is automatically reflected in every test that uses it; if the new boundary requires a different fixture size (e.g. `makeQueue(PREFETCH_THRESHOLD + 1)` for the "above threshold" case), the test fails at the right site with a clear signal.

**Exporting constants from production modules:** a constant used in tests must be exported from the production module. If the constant was previously unexported, add the `export` keyword rather than duplicating the value in the test file. The test's import is a compile-time link — a future rename or removal of the constant becomes a compile error rather than silent mismatch.

**Prefer named arithmetic expressions over magic offsets:** when the test exercises a value adjacent to the boundary (e.g. "one above threshold"), write `PREFETCH_THRESHOLD + 1` rather than a bare `6`. The arithmetic makes the intent self-documenting:

```ts
const initial = makeQueue(PREFETCH_THRESHOLD + 5); // clearly "above threshold"
const initial = makeQueue(PREFETCH_THRESHOLD);      // clearly "at threshold"
const initial = makeQueue(PREFETCH_THRESHOLD - 1);  // clearly "below threshold"
```

**Scope:** this rule applies to any numeric or string constant that defines a boundary the test is intended to exercise — page sizes, limits, timeouts, retry counts, thresholds. It does not apply to incidental fixture values (e.g. the exact text content of a card's front, a specific date string) that are not meaningful boundaries. Reference: `frontend/src/app/learn/[cardgroupId]/learn-client.test.tsx` importing `LEARN_PAGE_LIMIT` and `PREFETCH_THRESHOLD` to verify the background prefetch boundary condition.
