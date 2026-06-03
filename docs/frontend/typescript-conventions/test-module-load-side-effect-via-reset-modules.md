# Test module-load side effects via `vi.resetModules()` + dynamic `import()`

> Part of the [frontend TypeScript conventions](../typescript-conventions.md). See the index for related chapters.

## Why

A regular `import` caches the module. Any side effect that runs at module-load
time — a `console.warn` when a feature is absent, a singleton built from
`globalThis.crypto` — fires once before any test can stub globals. Subsequent
tests share the cached instance and cannot re-trigger the side effect.
`vi.stubGlobal("crypto", undefined)` in `beforeEach` has no effect on code that
already captured the global at evaluation time.

## What

The pattern forces a fresh module evaluation for each test that needs to observe
the side effect:

1. **`vi.resetModules()`** — clears the module registry so the next dynamic
   `import()` re-evaluates the module from scratch.
2. **`vi.stubGlobal(...)`** — sets the global to the value the test needs
   (e.g. `undefined` to simulate a missing Web Crypto API) **after** the
   registry is cleared but **before** the dynamic import.
3. **`await import("...")`** — triggers fresh module evaluation with the stub
   already in place.

Bundle these three steps into a `loadFresh()` helper when a `describe` block
contains multiple tests that all need a fresh load:

```ts
async function loadFresh(): Promise<typeof import("./my-module")["exportedFn"]> {
  vi.stubGlobal("crypto", undefined);
  const { exportedFn } = await import("./my-module");
  return exportedFn;
}
```

Call `vi.resetModules()` in `beforeEach` (not inside `loadFresh`) so the
registry is always cleared before the test body runs. Restore the stubbed
global in `afterEach` unconditionally to prevent cross-test pollution.

## Worked example

From `frontend/src/lib/observability/request-id.test.ts`:

```ts
describe("newRequestId — Web Crypto fallback path", () => {
  const realCrypto = globalThis.crypto;

  beforeEach(() => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
    vi.resetModules();
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.stubGlobal("crypto", realCrypto);
  });

  async function loadFresh(): Promise<typeof import("./request-id")["newRequestId"]> {
    vi.stubGlobal("crypto", undefined);
    const { newRequestId: fn } = await import("./request-id");
    return fn;
  }

  it("emits the module-load warn exactly once when crypto is undefined and still returns a UUID v7 string", async () => {
    const newRequestIdFresh = await loadFresh();

    expect(console.warn).toHaveBeenCalledTimes(1);
    expect(console.warn).toHaveBeenCalledWith(
      "[request-id] Web Crypto unavailable; falling back to Math.random-derived IDs (collision resistance reduced)",
    );

    const id = newRequestIdFresh();
    expect(id).toMatch(UUID_V7_RE);
  });

  it("produces 20 unique values on the fallback path", async () => {
    const newRequestIdFresh = await loadFresh();
    const ids = Array.from({ length: 20 }, newRequestIdFresh);
    expect(new Set(ids).size).toBe(20);
  });
});
```

## Gotchas

- **Ordering is strict.** `vi.resetModules()` must run before `vi.stubGlobal()`
  and before the dynamic `import()`. The safe order is always: reset → stub →
  import. Any other order risks the module being served from cache before the
  stub takes effect.

- **`afterEach` restore is mandatory.** Omitting `vi.stubGlobal("crypto", realCrypto)`
  (or `vi.unstubAllGlobals()`) in `afterEach` leaves the stub active for
  subsequent tests that do not expect the global to be absent, causing
  invisible flaky failures.

## See also

- `frontend/src/lib/observability/request-id.test.ts` — canonical `loadFresh()` usage.
- `frontend/src/lib/supabase/middleware.test.ts` — `vi.hoisted` + `async importOriginal` partial module replacement: spy on a single named export (`buildHtmlCsp`) while keeping the module's other exports real.
