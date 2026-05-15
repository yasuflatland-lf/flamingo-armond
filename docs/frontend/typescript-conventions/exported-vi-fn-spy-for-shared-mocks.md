# Exported `vi.fn()` spy for shared `vi.mock` factories

> Part of the [frontend TypeScript conventions](../typescript-conventions.md). See the index for related chapters.

A shared module-mock util (e.g. `frontend/__tests__/utils/mock-supabase.ts`) lets every test file that depends on the same module reuse one factory shape. The naive form returns a fresh implementation on every call:

```ts
// In mock-supabase.ts — naive.
// Note: this internal factory builds the mock client shape.
// Only mockCreateSupabaseServerClient (shown below) is exported for test vi.mock usage.
function mockSupabaseServerClientFactory() {
  return {
    auth: {
      getClaims: () => Promise.resolve({ data: { claims: state.claims }, error: null }),
    },
  };
}
```

Tests that need to assert `expect(supabase.auth.getClaims).not.toHaveBeenCalled()` cannot do so because the inline arrow `() => Promise.resolve(...)` is a fresh function on every factory invocation — it has no `mock` property and Vitest does not record call counts on it. Wrap the callable in `vi.fn()` to expose call counts:

```ts
// Correct — getClaims is a vi.fn() spy.
let latestGetClaimsSpy: ReturnType<typeof vi.fn<() => Promise<GetClaimsResult>>> | null = null;

export function mockSupabaseServerClient(): MockSupabaseServerClient {
  const getClaimsSpy = vi.fn<() => Promise<GetClaimsResult>>(() =>
    Promise.resolve(/* read state ... */),
  );
  latestGetClaimsSpy = getClaimsSpy;
  return { auth: { getClaims: getClaimsSpy /* ... */ } };
}

export function getMockGetClaimsSpy(): ReturnType<typeof vi.fn<() => Promise<GetClaimsResult>>> {
  if (latestGetClaimsSpy == null) {
    latestGetClaimsSpy = vi.fn<() => Promise<GetClaimsResult>>();
  }
  return latestGetClaimsSpy;
}
```

A fresh spy is installed on every factory invocation so previous spies do not retain stale closures over module-scoped state. The accessor returns the latest spy, materialising a fresh uncalled one if no factory invocation has happened yet (the convenience that masks regressions — see below).

## Pair the callable spy with a factory spy

The lazy-materialisation in `getMockGetClaimsSpy()` is a footgun: when the layout under test short-circuits before invoking `createSupabaseServerClient()` (e.g. on `/login` or `/onboarding` bypass branches), no factory call ever happens, the latest spy reference is `null`, and `getMockGetClaimsSpy()` materialises a fresh uncalled spy on demand. The test's `expect(getMockGetClaimsSpy()).not.toHaveBeenCalled()` assertion is then trivially true — it cannot distinguish "the layout short-circuited" from "the layout called supabase but the test imported the wrong path".

Catch this by exporting the **factory itself** as a `vi.fn()` and asserting on it from the test:

```ts
// In mock-supabase.ts.
export const mockCreateSupabaseServerClient = vi.fn(() =>
  Promise.resolve(mockSupabaseServerClient()),
);

export function resetMockSupabase(): void {
  // ... reset state ...
  latestGetClaimsSpy = null;
  mockCreateSupabaseServerClient.mockClear();  // load-bearing
}
```

```ts
// In the test file.
vi.mock("@/lib/supabase/server", () => ({
  createSupabaseServerClient: mockCreateSupabaseServerClient,
}));

// /login bypass — neither supabase client construction nor getClaims should fire.
expect(mockCreateSupabaseServerClient).not.toHaveBeenCalled();
expect(getMockGetClaimsSpy()).not.toHaveBeenCalled();
```

The `mockCreateSupabaseServerClient.not.toHaveBeenCalled()` assertion is the load-bearing one: a regression where the layout starts calling supabase on `/login` (e.g. an accidental refactor that moves the bypass check below the supabase client construction) fails this assertion, while `getMockGetClaimsSpy().not.toHaveBeenCalled()` still trivially passes.

## When to lift to a shared util vs inline `vi.fn()`

Inline `vi.fn()` per test file is fine when only one test needs the call-count assertion. Lift to a shared util when:

1. Three or more test files mock the same module via the same factory shape.
2. The factory is non-trivial (multiple state knobs, multiple return shapes — see the `setMockSupabaseClaimsDataNull()` setter for the TOCTOU race shape).
3. Any test asserts call counts on the factory itself — the inline `vi.mock("path", () => ({ X: vi.fn(...) }))` form does not expose the captured spy to assertions outside the factory closure.

Reference: `frontend/__tests__/utils/mock-supabase.ts` (the canonical implementation), `frontend/src/app/layout.test.tsx` (the `/login` bypass test that motivates the factory spy).
