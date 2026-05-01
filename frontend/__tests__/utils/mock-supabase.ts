/**
 * Shared mock factory for `@/lib/supabase/server` (`createSupabaseServerClient`).
 *
 * The Supabase server client is consumed by Next.js server components and route
 * handlers via `const supabase = await createSupabaseServerClient()` followed by
 * `await supabase.auth.getUser()`. This util produces a stub with the same
 * shape, plus state knobs the test controls per case.
 *
 * IMPORTANT: `vi.mock` must be invoked at the **top of each test file** because
 * Vitest hoists `vi.mock` calls above imports. This util therefore exports only
 * the factory (`mockSupabaseServerClient`) and the state setters; tests wire
 * the mock themselves.
 *
 * Canonical usage in a test file:
 *
 * ```ts
 * import {
 *   mockSupabaseServerClient,
 *   resetMockSupabase,
 *   setMockSupabaseUser,
 * } from "./utils/mock-supabase";
 *
 * vi.mock("@/lib/supabase/server", () => ({
 *   createSupabaseServerClient: () => Promise.resolve(mockSupabaseServerClient()),
 * }));
 *
 * beforeEach(() => {
 *   resetMockSupabase();
 *   setMockSupabaseUser({ id: "u-1", email: "u1@test" });
 * });
 * ```
 *
 * Logged-out request: `setMockSupabaseUser(null)`.
 * Auth transport error: `setMockSupabaseUserError(new Error("boom"))`.
 */

export type MockSupabaseUser = {
  id: string;
  email?: string;
};

type MockSupabaseState = {
  user: MockSupabaseUser | null;
  error: Error | null;
};

// Module-scoped state. `resetMockSupabase` is the canonical way to clear it
// between tests; the factory reads from this single source of truth.
const state: MockSupabaseState = {
  user: null,
  error: null,
};

/**
 * Set the user that the next `supabase.auth.getUser()` call will return.
 * Pass `null` to simulate a logged-out request (the default after reset).
 */
export function setMockSupabaseUser(user: MockSupabaseUser | null): void {
  state.user = user;
  state.error = null;
}

/**
 * Inject an auth transport error into the next `supabase.auth.getUser()`
 * call. The factory returns `{ data: { user: null }, error }` so callers
 * exercise the auth-error branch (e.g. layouts that re-throw on `error`).
 */
export function setMockSupabaseUserError(err: Error): void {
  state.error = err;
}

/**
 * Reset to a clean slate (logged out, no error). Intended for `beforeEach`.
 */
export function resetMockSupabase(): void {
  state.user = null;
  state.error = null;
}

/**
 * Shape returned to consumers of `createSupabaseServerClient()`. Only the
 * surface that pages actually call is implemented; widen the type as new
 * surfaces are exercised by tests.
 */
export type MockSupabaseServerClient = {
  auth: {
    getUser: () => Promise<{
      data: { user: MockSupabaseUser | null };
      error: Error | null;
    }>;
  };
};

/**
 * Factory for the mocked server client. Call this from inside a test file's
 * `vi.mock("@/lib/supabase/server", ...)` factory. Each invocation reads the
 * current module-scoped state, so mutations via `setMockSupabaseUser` /
 * `setMockSupabaseUserError` between renders are honoured.
 */
export function mockSupabaseServerClient(): MockSupabaseServerClient {
  return {
    auth: {
      getUser: () =>
        Promise.resolve({
          data: { user: state.user },
          error: state.error,
        }),
    },
  };
}
