/**
 * Lift GraphQL extension codes from an arbitrary Apollo / network error for
 * warn payloads. We do NOT trust err.message — backend messages may echo user
 * input — but extensions.code is a fixed enum from the server's resolver layer
 * and safe to log. The shape check is permissive: if the underlying transport
 * surfaces graphQLErrors as a property on the Error, pluck them; otherwise
 * return an empty list. This deliberately does not reuse parseGqlErrors because
 * that helper is keyed on the literal "GraphQL errors: " message prefix, which
 * not every transport produces.
 */
export function liftGraphQLCodes(err: unknown): string[] {
  if (err == null || typeof err !== "object") return [];
  const maybe = (err as { graphQLErrors?: unknown }).graphQLErrors;
  if (!Array.isArray(maybe)) return [];
  const codes: string[] = [];
  for (const entry of maybe) {
    const code = (entry as { extensions?: { code?: unknown } } | null | undefined)?.extensions
      ?.code;
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
