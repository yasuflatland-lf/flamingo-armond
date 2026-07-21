import { headers } from "next/headers";
import { redirect } from "next/navigation";

/** Result classification of the middleware's getClaims() call. */
export type AuthStatus = "authenticated" | "anonymous" | "stale" | "error";

/**
 * Server-internal request headers set by middleware and consumed by RSC via
 * next/headers. They are NEVER written onto the response (same posture as
 * x-nonce). The browser must never see them.
 */
export const AUTH_STATUS_HEADER = "x-auth-status";
export const USER_EMAIL_HEADER = "x-user-email";
export const USER_IS_ADMIN_HEADER = "x-user-is-admin";

/** The full set of forwarded identity header names (order pinned by auth-status.test.ts). */
export const IDENTITY_HEADERS = [
  AUTH_STATUS_HEADER,
  USER_EMAIL_HEADER,
  USER_IS_ADMIN_HEADER,
] as const;

export type AuthContext = {
  status: AuthStatus;
  email: string | null;
  /** UI hint only. The real admin gate is app/admin/layout.tsx. */
  isAdmin: boolean;
};

const VALID_STATUS: ReadonlySet<string> = new Set(["authenticated", "anonymous", "stale", "error"]);

/**
 * Reads the forwarded identity headers into a typed context. Defaults to the
 * safe "anonymous" classification when the header is absent or malformed, so a
 * dropped header degrades to the anonymous shell / a protected-page redirect
 * rather than leaking an authenticated view.
 */
export function readAuthContext(requestHeaders: Headers): AuthContext {
  const raw = requestHeaders.get(AUTH_STATUS_HEADER);
  const status: AuthStatus =
    raw != null && VALID_STATUS.has(raw) ? (raw as AuthStatus) : "anonymous";
  const email = requestHeaders.get(USER_EMAIL_HEADER) || null;
  const isAdmin = requestHeaders.get(USER_IS_ADMIN_HEADER) === "true";
  return { status, email, isAdmin };
}

/**
 * The RSC auth gate: reads the forwarded identity headers and sends the visitor
 * to `target` unless the middleware classified the request as "authenticated"
 * (i.e. also on "anonymous", "stale" and "error").
 *
 * The redirect target is an explicit argument rather than a literal baked into
 * the helper, so every page states in reviewable form where its unauthenticated
 * visitor lands: `/login` for the ordinary signed-in surfaces, `/` for the admin
 * surfaces. See
 * docs/frontend/rsc-error-handling/unauthenticated-redirect-target-must-be-login.md
 * for why those are the only two sanctioned targets.
 *
 * Returns the AuthContext so a page that also reads `email` / `isAdmin` can bind
 * the result instead of parsing the headers a second time. `redirect()` throws,
 * so the returned context is always an authenticated one.
 *
 * Call this OUTSIDE any <Suspense> boundary — Next.js cannot redirect mid-stream,
 * and a redirect from within a suspended subtree flashes the skeleton first.
 */
export async function requireAuthenticated(target: string): Promise<AuthContext> {
  const auth = readAuthContext(await headers());
  if (auth.status !== "authenticated") redirect(target);
  return auth;
}
