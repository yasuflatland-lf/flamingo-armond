import "server-only";
import { redirect } from "next/navigation";

/**
 * If `err` carries an UNAUTHENTICATED GraphQL code, redirect to `target`.
 * Otherwise rethrow `err` so the error boundary handles it.
 *
 * Returns `never`: it always throws (either via `redirect()` or rethrow).
 * Intended for use inside RSC `catch` blocks around `gqlFetch` calls.
 */
export function redirectIfUnauthenticated(err: unknown, target: string): never {
  const msg = err instanceof Error ? err.message : String(err);
  if (msg.includes("UNAUTHENTICATED")) redirect(target);
  throw err;
}
