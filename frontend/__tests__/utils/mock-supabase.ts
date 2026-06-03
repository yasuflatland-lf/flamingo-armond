import { vi } from "vitest";

/**
 * Shared mock factory for `@/lib/supabase/server` (`createSupabaseServerClient`).
 *
 * Scope: the Next.js middleware (`src/lib/supabase/middleware.ts`) calls
 * `createSupabaseServerClient()` followed by `supabase.auth.getUser()` directly.
 * All page RSCs (including `app/admin/*` and `app/login/page.tsx`) now read
 * identity from middleware-forwarded request headers via `readAuthContext` and
 * do NOT use this factory; those tests mock `next/headers` instead.
 *
 * IMPORTANT: `vi.mock` must be invoked at the **top of each test file** because
 * Vitest hoists `vi.mock` calls above imports. This util therefore exports only
 * `mockCreateSupabaseServerClient` and the state setters; tests wire
 * the mock themselves.
 *
 * Canonical usage in a test file:
 *
 * ```ts
 * import {
 *   mockCreateSupabaseServerClient,
 *   resetMockSupabase,
 *   setMockSupabaseUser,
 * } from "./utils/mock-supabase";
 *
 * vi.mock("@/lib/supabase/server", () => ({
 *   createSupabaseServerClient: mockCreateSupabaseServerClient,
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
 * call. Only `state.error` is set; `state.user` is left unchanged. To get
 * the typical `{ data: { user: null }, error }` shape, call
 * `resetMockSupabase()` first (which the canonical `beforeEach` already does),
 * then call this function. If a prior `setMockSupabaseUser(...)` call was made
 * in the same test, the factory will return that user alongside the error.
 */
export function setMockSupabaseUserError(err: Error): void {
  state.error = err;
}

/**
 * Spy for the `createSupabaseServerClient` factory call itself. Test files
 * wire this into their `vi.mock("@/lib/supabase/server", ...)` factory so the
 * factory invocation is observable:
 *
 * ```ts
 * vi.mock("@/lib/supabase/server", () => ({
 *   createSupabaseServerClient: mockCreateSupabaseServerClient,
 * }));
 * ```
 */
export const mockCreateSupabaseServerClient = vi.fn(() =>
  Promise.resolve(mockSupabaseServerClient()),
);

/**
 * Reset to a clean slate (logged out, no errors). Intended for `beforeEach`.
 */
export function resetMockSupabase(): void {
  state.user = null;
  state.error = null;
  mockCreateSupabaseServerClient.mockClear();
}

/**
 * Shape returned to consumers of `createSupabaseServerClient()`. Only the
 * surface that pages actually call is implemented; widen the type as new
 * surfaces are exercised by tests.
 */
type MockSupabaseServerClient = {
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
function mockSupabaseServerClient(): MockSupabaseServerClient {
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
