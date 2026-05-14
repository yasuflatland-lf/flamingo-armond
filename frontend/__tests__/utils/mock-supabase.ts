import { vi } from "vitest";

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
 * The `getClaims` callable is exposed as a `vi.fn()` spy so tests can assert
 * call counts (e.g. `expect(getMockGetClaimsSpy()).not.toHaveBeenCalled()` for
 * cases where the layout must short-circuit before reaching the JWT lookup).
 * Each call to `mockSupabaseServerClient()` installs a fresh spy; the latest
 * one is accessible via `getMockGetClaimsSpy()`.
 *
 * Canonical usage in a test file:
 *
 * ```ts
 * import {
 *   getMockGetClaimsSpy,
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
 * TOCTOU race shape `{ data: null, error: null }`: `setMockSupabaseClaimsDataNull()`.
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
  // When true, the factory returns `{ data: null, error: null }` from
  // getClaims (the SDK's third return shape — a TOCTOU race where the session
  // vanished between getUser() and getClaims()). This is structurally distinct
  // from `claims: null`, which returns `{ data: { claims: null }, error: null }`.
  claimsDataNull: boolean;
};

// Module-scoped state. `resetMockSupabase` is the canonical way to clear it
// between tests; the factory reads from this single source of truth.
const state: MockSupabaseState = {
  user: null,
  error: null,
  claims: null,
  claimsError: null,
  claimsDataNull: false,
};

/**
 * Spy installed on the most recent `mockSupabaseServerClient()` invocation's
 * `getClaims`. Tests assert call counts (e.g. "must not be called on the
 * /login bypass branch") via `getMockGetClaimsSpy()`.
 *
 * A fresh `vi.fn()` is installed on every factory call so that previous spies
 * do not retain references to stale state-driven closures. `resetMockSupabase`
 * clears this reference along with the rest of the state.
 */
type GetClaimsResult = {
  data: { claims: MockSupabaseClaims | null } | null;
  error: Error | null;
};
let latestGetClaimsSpy: ReturnType<typeof vi.fn<() => Promise<GetClaimsResult>>> | null = null;

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
 *
 * Note: this produces `{ data: { claims: null }, error: null }`. To simulate
 * the SDK's third return shape `{ data: null, error: null }` (the TOCTOU race
 * window), use `setMockSupabaseClaimsDataNull()` instead.
 */
export function setMockSupabaseClaims(claims: MockSupabaseClaims | null): void {
  state.claims = claims;
  state.claimsError = null;
  state.claimsDataNull = false;
}

/**
 * Inject an error into the next `supabase.auth.getClaims()` call.
 * Only `state.claimsError` is set; `state.claims` is left unchanged.
 */
export function setMockSupabaseClaimsError(err: Error): void {
  state.claimsError = err;
}

/**
 * Make the next `supabase.auth.getClaims()` call return the SDK's third return
 * shape: `{ data: null, error: null }`. This models the TOCTOU race where the
 * session vanished between `getUser()` and `getClaims()` — the layout must
 * degrade `isAdmin` to false and warn without throwing.
 */
export function setMockSupabaseClaimsDataNull(): void {
  state.claimsDataNull = true;
  state.claimsError = null;
}

/**
 * Return the `vi.fn()` spy attached to the most recent
 * `mockSupabaseServerClient()` invocation's `getClaims`. Tests use this to
 * assert call counts — e.g. `expect(getMockGetClaimsSpy()).not.toHaveBeenCalled()`
 * for cases where the layout must short-circuit before reaching the JWT lookup.
 *
 * If no factory invocation has occurred yet (e.g. the layout short-circuited
 * before calling `createSupabaseServerClient()` on the /login bypass branch),
 * a fresh uncalled spy is materialised so the typical `not.toHaveBeenCalled()`
 * assertion remains meaningful: the property under test is "getClaims was never
 * invoked", and an uninvoked factory trivially satisfies it.
 */
export function getMockGetClaimsSpy(): ReturnType<typeof vi.fn<() => Promise<GetClaimsResult>>> {
  if (latestGetClaimsSpy == null) {
    latestGetClaimsSpy = vi.fn<() => Promise<GetClaimsResult>>();
  }
  return latestGetClaimsSpy;
}

/**
 * Reset to a clean slate (logged out, no errors, no claims). Intended for `beforeEach`.
 */
export function resetMockSupabase(): void {
  state.user = null;
  state.error = null;
  state.claims = null;
  state.claimsError = null;
  state.claimsDataNull = false;
  latestGetClaimsSpy = null;
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
    getClaims: ReturnType<typeof vi.fn<() => Promise<GetClaimsResult>>>;
  };
};

/**
 * Factory for the mocked server client. Call this from inside a test file's
 * `vi.mock("@/lib/supabase/server", ...)` factory. Each invocation reads the
 * current module-scoped state, so mutations via `setMockSupabaseUser` /
 * `setMockSupabaseUserError` / `setMockSupabaseClaims` / `setMockSupabaseClaimsError` /
 * `setMockSupabaseClaimsDataNull` between renders are honoured.
 *
 * The `getClaims` callable is a fresh `vi.fn()` per invocation; the latest spy
 * is exposed via `getMockGetClaimsSpy()` for call-count assertions.
 */
export function mockSupabaseServerClient(): MockSupabaseServerClient {
  const getClaimsSpy = vi.fn<() => Promise<GetClaimsResult>>(() =>
    Promise.resolve(
      state.claimsDataNull
        ? { data: null, error: null }
        : { data: { claims: state.claims }, error: state.claimsError },
    ),
  );
  latestGetClaimsSpy = getClaimsSpy;
  return {
    auth: {
      getUser: () =>
        Promise.resolve({
          data: { user: state.user },
          error: state.error,
        }),
      getClaims: getClaimsSpy,
    },
  };
}
