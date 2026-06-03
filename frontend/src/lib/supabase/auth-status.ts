/** Result classification of the middleware's getUser() call. */
export type AuthStatus = "authenticated" | "anonymous" | "stale" | "error";

/**
 * Server-internal request headers set by middleware and consumed by RSC via
 * next/headers. They are NEVER written onto the response (same posture as
 * x-nonce). The browser must never see them.
 */
export const AUTH_STATUS_HEADER = "x-auth-status";
export const USER_EMAIL_HEADER = "x-user-email";
export const USER_IS_ADMIN_HEADER = "x-user-is-admin";

/** The full set, in strip/iteration order. */
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
export function readAuthContext(headers: Headers): AuthContext {
  const raw = headers.get(AUTH_STATUS_HEADER);
  const status: AuthStatus = raw != null && VALID_STATUS.has(raw) ? (raw as AuthStatus) : "anonymous";
  const email = headers.get(USER_EMAIL_HEADER) || null;
  const isAdmin = headers.get(USER_IS_ADMIN_HEADER) === "true";
  return { status, email, isAdmin };
}
