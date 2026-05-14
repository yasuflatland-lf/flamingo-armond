/**
 * Shared mock factory for `@/lib/supabase/server` (`createSupabaseServerClient`).
 *
 * The Supabase server client is consumed by Next.js server components and route
 * handlers via `const supabase = await createSupabaseServerClient()` followed by
 * `await supabase.auth.getUser()` and `await supabase.auth.getClaims()`. This
 * util produces a stub with the same shape, plus state knobs the test controls
 * per case.
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
 *   setMockSupabaseClaims,
 * } from "./utils/mock-supabase";
 *
 * vi.mock("@/lib/supabase/server", () => ({
 *   createSupabaseServerClient: () => Promise.resolve(mockSupabaseServerClient()),
 * }));
 *
 * beforeEach(() => {
 *   resetMockSupabase();
 *   setMockSupabaseUser({ id: "u-1", email: "u1@test" });
 *   setMockSupabaseClaims({ app_metadata: { role: "admin" } });
 * });
 * ```
 *
 * Logged-out request: `setMockSupabaseUser(null)`.
 * Auth transport error: `setMockSupabaseUserError(new Error("boom"))`.
 * Claims error: `setMockSupabaseClaimsError(new Error("boom"))`.
 */

export type MockSupabaseUser = {
  id: string;
  email?: string;
};

export type MockSupabaseClaims = {
  app_metadata?: { role?: string; [key: string]: unknown };
  [key: string]: unknown;
};

type MockSupabaseState = {
  user: MockSupabaseUser | null;
  error: Error | null;
  claims: MockSupabaseClaims | null;
  claimsError: Error | null;
};

// Module-scoped state. `resetMockSupabase` is the canonical way to clear it
// between tests; the factory reads from this single source of truth.
const state: MockSupabaseState = {
  user: null,
  error: null,
  claims: null,
  claimsError: null,
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
 * Set the claims that the next `supabase.auth.getClaims()` call will return.
 * Pass `null` to simulate a missing or empty claims result (the default after reset).
 */
export function setMockSupabaseClaims(claims: MockSupabaseClaims | null): void {
  state.claims = claims;
  state.claimsError = null;
}

/**
 * Inject an error into the next `supabase.auth.getClaims()` call.
 * Only `state.claimsError` is set; `state.claims` is left unchanged.
 */
export function setMockSupabaseClaimsError(err: Error): void {
  state.claimsError = err;
}

/**
 * Reset to a clean slate (logged out, no errors, no claims). Intended for `beforeEach`.
 */
export function resetMockSupabase(): void {
  state.user = null;
  state.error = null;
  state.claims = null;
  state.claimsError = null;
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
    getClaims: () => Promise<{
      data: { claims: MockSupabaseClaims | null };
      error: Error | null;
    }>;
  };
};

/**
 * Factory for the mocked server client. Call this from inside a test file's
 * `vi.mock("@/lib/supabase/server", ...)` factory. Each invocation reads the
 * current module-scoped state, so mutations via `setMockSupabaseUser` /
 * `setMockSupabaseUserError` / `setMockSupabaseClaims` / `setMockSupabaseClaimsError`
 * between renders are honoured.
 */
export function mockSupabaseServerClient(): MockSupabaseServerClient {
  return {
    auth: {
      getUser: () =>
        Promise.resolve({
          data: { user: state.user },
          error: state.error,
        }),
      getClaims: () =>
        Promise.resolve({
          data: { claims: state.claims },
          error: state.claimsError,
        }),
    },
  };
}
