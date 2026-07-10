/**
 * Pure Apollo/GraphQL error-parsing helpers shared across forms.
 *
 * Contract:
 *  - getBackendFieldErrors:  BAD_USER_INPUT errors that carry a "field" extension
 *    are returned as { fieldName: message }.  All other errors are ignored.
 *  - getBackendErrorBanner:  Returns a user-facing banner string for non-field
 *    errors.  Priority: INTERNAL > UNAUTHENTICATED > first non-field GQL error.
 *    Network / non-CombinedGraphQLErrors → generic network message.
 *  - classifyQueryError:  Classifies a query-level error into a typed result so
 *    callers can branch on FORBIDDEN / UNAUTHENTICATED without retrying.
 *
 * @see ./graphql-errors.ts for the parallel gqlFetch / RSC error classifiers
 * (the `"GraphQL errors: "`-prefixed-`Error` shape: isUnauthenticatedGraphQLError
 * etc.) plus liftGraphQLCodes. The two modules are a deliberate, CI-tested split
 * — do not merge them.
 */

import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { liftGraphQLCodes } from "./graphql-errors";

const NETWORK_ERROR = "Could not reach the server. Please try again.";

function extensionString(
  extensions: Record<string, unknown> | undefined,
  key: string,
): string | undefined {
  const value = extensions?.[key];
  return typeof value === "string" ? value : undefined;
}

/** Returns a map of field name to error message for BAD_USER_INPUT errors. */
export function getBackendFieldErrors(err: unknown): Record<string, string> {
  if (!CombinedGraphQLErrors.is(err)) return {};
  const out: Record<string, string> = {};
  for (const ge of err.errors) {
    const code = extensionString(ge.extensions, "code");
    const field = extensionString(ge.extensions, "field");
    if (code === "BAD_USER_INPUT" && field) {
      out[field] = ge.message;
    }
  }
  return out;
}

/**
 * Returns a user-facing banner message for non-field errors.
 * Priority: INTERNAL > UNAUTHENTICATED > first non-field GQL error > network error.
 * Returns undefined when every error is a field-level BAD_USER_INPUT.
 */
export function getBackendErrorBanner(err: unknown): string | undefined {
  if (!err) return undefined;
  if (!CombinedGraphQLErrors.is(err)) return NETWORK_ERROR;
  let internalMsg: string | undefined;
  let authMsg: string | undefined;
  let firstNonFieldMsg: string | undefined;
  for (const ge of err.errors) {
    const code = extensionString(ge.extensions, "code");
    const field = extensionString(ge.extensions, "field");
    if (code === "INTERNAL") {
      internalMsg ??= ge.message;
    } else if (code === "UNAUTHENTICATED") {
      authMsg ??= "Your session expired. Please sign in again.";
    } else if (code !== "BAD_USER_INPUT" || !field) {
      // Skip field-level BAD_USER_INPUT; everything else is banner-worthy.
      firstNonFieldMsg ??= ge.message;
    }
  }
  return internalMsg ?? authMsg ?? firstNonFieldMsg;
}

/**
 * Typed result of classifying a query-level Apollo error.
 *
 * - `forbidden`:       FORBIDDEN code — user lacks the required role.
 * - `unauthenticated`: UNAUTHENTICATED code — session expired.
 * - `banner`:          Any other error; `message` carries the user-facing string.
 */
export type QueryErrorKind =
  | { kind: "forbidden" }
  | { kind: "unauthenticated" }
  | { kind: "banner"; message: string };

/**
 * Classify a query-level Apollo error for callers that need to branch on
 * FORBIDDEN (no retry, permission copy) or UNAUTHENTICATED (redirect/re-login)
 * separately from generic errors that warrant a Retry banner.
 *
 * Returns `null` when `err` is falsy.
 */
export function classifyQueryError(err: unknown): QueryErrorKind | null {
  if (!err) return null;
  if (CombinedGraphQLErrors.is(err)) {
    for (const ge of err.errors) {
      const code = extensionString(ge.extensions, "code");
      if (code === "FORBIDDEN") return { kind: "forbidden" };
      if (code === "UNAUTHENTICATED") return { kind: "unauthenticated" };
    }
  }
  return {
    kind: "banner",
    message: getBackendErrorBanner(err) ?? "An unexpected error occurred. Please try again.",
  };
}

/**
 * Auth-relevant classification of a mutation-level Apollo error.
 *
 * - `forbidden`:       FORBIDDEN code — caller lacks the required role.
 * - `unauthenticated`: UNAUTHENTICATED code — session expired.
 * - `other`:           Any other GraphQL error or a network/transport failure.
 */
export type MutationAuthErrorKind = "forbidden" | "unauthenticated" | "other";

/**
 * Classify a mutation-level Apollo error by its auth-relevant extensions.code.
 * Returns the kind so callers can fold it into either a translated banner string
 * (see `mutationAuthBanner`) or a discriminated state key, without re-implementing
 * the FORBIDDEN / UNAUTHENTICATED detection per screen.
 *
 * Precedence mirrors `classifyQueryError`: the first auth error encountered in the
 * `errors` array wins. Anything that is not a CombinedGraphQLErrors (network /
 * transport failure, falsy value) classifies as `other`.
 */
export function classifyMutationAuthError(err: unknown): MutationAuthErrorKind {
  if (CombinedGraphQLErrors.is(err)) {
    for (const ge of err.errors) {
      const code = extensionString(ge.extensions, "code");
      if (code === "FORBIDDEN") return "forbidden";
      if (code === "UNAUTHENTICATED") return "unauthenticated";
    }
  }
  return "other";
}

/**
 * The auth-relevant kinds a caught mutation error can resolve to once the
 * non-auth `"other"` case is folded into `rejected`. This is the auth-kind
 * union the mutation hooks surface to their callers.
 */
export type MutationAuthKind = Exclude<MutationAuthErrorKind, "other">;

/**
 * Discriminated outcome of folding a caught mutation error: a typed `auth`
 * outcome for FORBIDDEN / UNAUTHENTICATED, otherwise `rejected`.
 *
 * `Kind` is the caller's auth-kind union and defaults to the full
 * `"forbidden" | "unauthenticated"` pair. `classifyMutationAuthError` returns
 * the literal `"forbidden"` / `"unauthenticated"` strings, which structurally
 * satisfy any `Kind` that contains them, so the narrowing in
 * `classifyAndLogAuthOutcome` is sound.
 */
export type MutationCatchOutcome<Kind extends MutationAuthKind = MutationAuthKind> =
  | { status: "auth"; kind: Kind }
  | { status: "rejected" };

/**
 * Fold a caught mutation-level error into the discriminated `auth` / `rejected`
 * outcome shared by the auth-classifying mutation hooks. This single-sources the
 * `catch`-block boilerplate those hooks repeat:
 *
 *  - FORBIDDEN / UNAUTHENTICATED → `{ status: "auth", kind }` so the caller can
 *    pick sign-in / permission toast copy.
 *  - Anything else (a non-auth GraphQL error, a transport failure) → emits a
 *    scoped structured `console.warn` and returns `{ status: "rejected" }`.
 *
 * The warn carries `err.name` and the lifted `extensions.code` list (via
 * `liftGraphQLCodes`) plus any caller-supplied `extra` context (e.g. an entity
 * id). `err.message` is intentionally omitted — backend messages may echo user
 * input. `scope` and `op` parameterize the per-hook warn prefix
 * (`"[scope] op rejected"`) so each hook keeps its own telemetry label.
 *
 * `Kind` defaults to the full `"forbidden" | "unauthenticated"` union but can
 * be narrowed by the caller's auth-kind union (some hooks surface both, some
 * only expect `unauthenticated`).
 */
export function classifyAndLogAuthOutcome<Kind extends MutationAuthKind = MutationAuthKind>(
  err: unknown,
  scope: string,
  op: string,
  extra?: Record<string, unknown>,
): MutationCatchOutcome<Kind> {
  const kind = classifyMutationAuthError(err);
  if (kind !== "other") return { status: "auth", kind: kind as Kind };
  console.warn(`[${scope}] ${op} rejected`, {
    ...extra,
    name: err instanceof Error ? err.name : "unknown",
    codes: liftGraphQLCodes(err),
  });
  return { status: "rejected" };
}

/**
 * Fold a mutation-level Apollo error into one of three caller-supplied banner
 * strings. The caller passes already-translated copy so the i18n namespace stays
 * at the call site; FORBIDDEN and UNAUTHENTICATED are detected via
 * `classifyMutationAuthError`, and everything else falls through to `fallback`.
 */
export function mutationAuthBanner(
  err: unknown,
  copy: { forbidden: string; unauthenticated: string; fallback: string },
): string {
  switch (classifyMutationAuthError(err)) {
    case "forbidden":
      return copy.forbidden;
    case "unauthenticated":
      return copy.unauthenticated;
    default:
      return copy.fallback;
  }
}
