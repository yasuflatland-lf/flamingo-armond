import { CombinedGraphQLErrors } from "@apollo/client/errors";

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
