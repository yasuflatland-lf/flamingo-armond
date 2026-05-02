/**
 * Returns true when `err` is the synthesized GraphQL error thrown by gqlFetch
 * (`server.ts`) and at least one entry in the parsed errors array carries
 * `extensions.code === "UNAUTHENTICATED"`. Substring matching is intentionally
 * NOT used — see .claude/rules/frontend-rsc-error-handling.md.
 */
export function isUnauthenticatedGraphQLError(err: unknown): boolean {
  if (!(err instanceof Error)) return false;
  const prefix = "GraphQL errors: ";
  if (!err.message.startsWith(prefix)) return false;
  try {
    const parsed = JSON.parse(err.message.slice(prefix.length)) as Array<{
      extensions?: { code?: string };
    }>;
    return (
      Array.isArray(parsed) &&
      parsed.some((e) => e?.extensions?.code === "UNAUTHENTICATED")
    );
  } catch {
    return false;
  }
}
