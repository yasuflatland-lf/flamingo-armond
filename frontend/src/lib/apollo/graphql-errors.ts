/**
 * Returns the structured GraphQL error array thrown by gqlFetch (`server.ts`)
 * when `err.message` carries the "GraphQL errors: ..." prefix; otherwise null.
 * Centralised so every code-extracting helper below shares one parser.
 */
function parseGqlErrors(err: unknown): Array<{ extensions?: { code?: string } }> | null {
  if (!(err instanceof Error)) return null;
  const prefix = "GraphQL errors: ";
  if (!err.message.startsWith(prefix)) return null;
  try {
    const parsed = JSON.parse(err.message.slice(prefix.length));
    return Array.isArray(parsed) ? parsed : null;
  } catch {
    return null;
  }
}

/**
 * Returns true when `err` is the synthesized GraphQL error thrown by gqlFetch
 * (`server.ts`) and at least one entry in the parsed errors array carries
 * `extensions.code === "UNAUTHENTICATED"`. Substring matching is intentionally
 * NOT used — see .claude/rules/frontend-rsc-error-handling.md.
 */
export function isUnauthenticatedGraphQLError(err: unknown): boolean {
  return parseGqlErrors(err)?.some((e) => e?.extensions?.code === "UNAUTHENTICATED") ?? false;
}

/**
 * Counterpart to `isUnauthenticatedGraphQLError` for the FORBIDDEN code. Used
 * by RSCs that need to distinguish an authorization failure (route the caller
 * back into the section's listing) from an authentication failure (route to
 * /login).
 */
export function isForbiddenGraphQLError(err: unknown): boolean {
  return parseGqlErrors(err)?.some((e) => e?.extensions?.code === "FORBIDDEN") ?? false;
}
