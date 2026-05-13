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
