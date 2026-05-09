import type { AuthError } from "@supabase/supabase-js";

/**
 * Returns true when `err` represents a stale JWT — the token is structurally
 * valid but the user it references has been deleted from auth.users. Treat it
 * the same as AuthSessionMissingError: redirect to /login, do not 500.
 */
export function isStaleSessionError(err: AuthError | null): boolean {
  return (
    err?.name === "AuthApiError" &&
    typeof err.message === "string" &&
    err.message.includes("does not exist")
  );
}

/**
 * Returns true when `err` is ignorable for RSC auth-gate purposes.
 * Both missing-session (anonymous request) and stale-session (deleted user)
 * should fall through to the `!user` redirect rather than throwing.
 */
export function isIgnorableAuthError(err: AuthError | null): boolean {
  return err === null || err.name === "AuthSessionMissingError" || isStaleSessionError(err);
}
