# Import shared mock identifiers BEFORE `vi.mock(...)` — Vitest hoisting + TDZ

> Part of the [frontend TypeScript conventions](../typescript-conventions.md). See the index for related chapters.

Vitest hoists `vi.mock(...)` calls to the top of the file at compile time, ahead of all `import` statements, so the mock is registered before any consumer of the mocked module evaluates. The naive pattern of inlining `vi.fn()` inside the factory works because the factory body is itself hoisted with the call:

```ts
// Self-contained — works.
vi.mock("@/lib/supabase/server", () => ({
  createSupabaseServerClient: vi.fn(() => Promise.resolve({ auth: { /* ... */ } })),
}));
```

The problem appears when the factory references an **identifier imported from another file** — e.g. a shared spy exported from a test utility:

```ts
// At-risk pattern — the factory references an imported symbol.
import { mockCreateSupabaseServerClient } from "./utils/mock-supabase";

vi.mock("@/lib/supabase/server", () => ({
  createSupabaseServerClient: mockCreateSupabaseServerClient,
}));
```

Vitest hoists the `vi.mock` call above the `import` statement, but the **factory function body** is not invoked until the mock is consumed. By that time the import has completed and `mockCreateSupabaseServerClient` is bound. The arrangement therefore works in practice — but only because the factory captures the binding **lazily** at call time, not at hoist time. Reverse the order (or inline-evaluate the factory) and you hit a temporal dead zone (TDZ):

```ts
// TDZ — fails.
vi.mock("@/lib/supabase/server", {
  createSupabaseServerClient: mockCreateSupabaseServerClient,  // immediate evaluation
});
import { mockCreateSupabaseServerClient } from "./utils/mock-supabase";
```

Two safe patterns:

1. **Import the shared util first, then call `vi.mock` with a factory function** (the canonical shape — the factory body's lazy evaluation makes the import-then-vi.mock ordering safe at runtime, and reads naturally top-to-bottom):

   ```ts
   import { mockCreateSupabaseServerClient } from "./utils/mock-supabase";

   vi.mock("@/lib/supabase/server", () => ({
     createSupabaseServerClient: mockCreateSupabaseServerClient,
   }));

   import RootLayout from "@/app/layout";  // consumer of the mocked module
   ```

2. **Use `vi.hoisted(() => ...)`** to construct the spy *inside* a hoisted block, so the binding exists at hoist time:

   ```ts
   const { mockCreateSupabaseServerClient } = vi.hoisted(() => ({
     mockCreateSupabaseServerClient: vi.fn(/* ... */),
   }));

   vi.mock("@/lib/supabase/server", () => ({
     createSupabaseServerClient: mockCreateSupabaseServerClient,
   }));
   ```

Pattern 1 is preferred when the spy is genuinely shared across test files (so it must live in a separate util module that other test files also import). Pattern 2 is preferred when the spy is local to a single test file and `vi.hoisted` provides the same hoist-time guarantee without crossing a module boundary.

**The consumer of the mocked module (e.g. `import RootLayout from "@/app/layout"`) MUST come after the `vi.mock` call.** If the consumer is imported above the `vi.mock` line, the module graph resolves before the mock is registered and the real implementation leaks through. This is a separate constraint from the TDZ issue — see also [`docs/frontend/testing-convention-narrow-vs-broad-page-tests.md` § "Add `vi.mock` for every new child component at the top of the test file"](../testing-convention-narrow-vs-broad-page-tests.md).

Reference: `frontend/src/app/layout.test.tsx` (the import-then-`vi.mock`-then-import-consumer ordering at the top of the file).
