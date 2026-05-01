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
 */

import { CombinedGraphQLErrors } from "@apollo/client/errors";

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
