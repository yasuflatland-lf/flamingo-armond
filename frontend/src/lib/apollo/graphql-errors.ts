/**
 * gqlFetch / RSC error classifiers: parse the `"GraphQL errors: "`-prefixed
 * `Error` that gqlFetch (server.ts) throws and read extensions.code
 * (isUnauthenticatedGraphQLError / isForbiddenGraphQLError /
 * isBadUserInputGraphQLError). liftGraphQLCodes additionally lifts codes from an
 * Apollo-runtime CombinedGraphQLErrors for warn payloads. redirectIfAuthError
 * packages the classify-and-redirect half of an RSC catch arm.
 *
 * @see ./errors.ts for the parallel form-facing Apollo Client runtime
 * (CombinedGraphQLErrors) helpers (getBackendFieldErrors / getBackendErrorBanner
 * / classifyQueryError). The two modules are a deliberate, CI-tested split — do
 * not merge them.
 */

import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { redirect } from "next/navigation";

/**
 * Lift GraphQL extension codes from an arbitrary Apollo / network error for
 * warn payloads. We do NOT trust err.message — backend messages may echo user
 * input — but extensions.code is a fixed enum from the server's resolver layer
 * and safe to log. The shape check uses CombinedGraphQLErrors.is, which is the
 * canonical narrowing helper in Apollo Client v4 — the rejected error from a
 * useMutation / useQuery hook is an instance of that class when the server
 * responded with an `errors` array. Errors that are not CombinedGraphQLErrors
 * (e.g. transport / network failures) return an empty list so callers can fall
 * through to the generic warn path. This deliberately does not reuse
 * parseGqlErrors because that helper is keyed on the literal
 * "GraphQL errors: " message prefix produced by gqlFetch, not by the Apollo
 * Client runtime.
 */
export function liftGraphQLCodes(err: unknown): string[] {
  if (!CombinedGraphQLErrors.is(err)) return [];
  const codes: string[] = [];
  for (const entry of err.errors) {
    const code = entry?.extensions?.code;
    if (typeof code === "string") codes.push(code);
  }
  return codes;
}

function parseGqlErrors(err: unknown): Array<{ extensions?: { code?: string } }> | null {
  if (!(err instanceof Error)) return null;
  const prefix = "GraphQL errors: ";
  if (!err.message.startsWith(prefix)) return null;
  try {
    const parsed = JSON.parse(err.message.slice(prefix.length));
    return Array.isArray(parsed) ? parsed : null;
  } catch (e) {
    console.warn("[graphql-errors] failed to parse GraphQL error message", {
      name: e instanceof Error ? e.name : "unknown",
    });
    return null;
  }
}

export function isUnauthenticatedGraphQLError(err: unknown): boolean {
  return parseGqlErrors(err)?.some((e) => e?.extensions?.code === "UNAUTHENTICATED") ?? false;
}

export function isForbiddenGraphQLError(err: unknown): boolean {
  return parseGqlErrors(err)?.some((e) => e?.extensions?.code === "FORBIDDEN") ?? false;
}

export function isBadUserInputGraphQLError(err: unknown): boolean {
  return parseGqlErrors(err)?.some((e) => e?.extensions?.code === "BAD_USER_INPUT") ?? false;
}

/**
 * The auth half of an RSC `catch` arm: redirects to `target` when the rejected
 * gqlFetch carries UNAUTHENTICATED, and — when `options.forbidden` is set — also
 * when it carries FORBIDDEN. The admin surfaces are the only callers that fold
 * FORBIDDEN in, because a signed-in non-admin has to land in-app rather than at
 * sign-in; see
 * docs/frontend/rsc-error-handling/unauthenticated-redirect-target-must-be-login.md
 * for why the ordinary target is `/login` and why the admin surfaces diverge.
 *
 * The helper deliberately owns ONLY the classify-and-redirect decision. Logging,
 * re-throwing and any degrade-to-a-default fallback stay at the call site,
 * because the arms are not uniform: most log the redacted error name and
 * re-throw, while the secondary admin-only fetch in `app/profile/page.tsx`
 * degrades the slider to its default instead of re-throwing. A wrapper that also
 * owned the logging and the re-throw could not express that without a flag.
 *
 * `redirect()` throws a Next.js control-flow error, so nothing after a matched
 * classification runs; the `void` return keeps the call site readable as a plain
 * guard statement rather than an assignment.
 */
export function redirectIfAuthError(
  err: unknown,
  target: string,
  options?: { forbidden?: boolean },
): void {
  if (isUnauthenticatedGraphQLError(err)) redirect(target);
  if (options?.forbidden === true && isForbiddenGraphQLError(err)) redirect(target);
}
