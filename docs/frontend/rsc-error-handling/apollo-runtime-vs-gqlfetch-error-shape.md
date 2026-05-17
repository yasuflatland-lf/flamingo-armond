# `isUnauthenticatedGraphQLError` matches gqlFetch — not Apollo Client runtime errors

> Part of the [frontend RSC error handling](../../../.claude/rules/frontend-rsc-error-handling.md) rules.

`isUnauthenticatedGraphQLError` and `isForbiddenGraphQLError` in `frontend/src/lib/apollo/graphql-errors.ts` look like universal helpers but they are not. Each delegates to `parseGqlErrors`, which begins:

```ts
const prefix = "GraphQL errors: ";
if (!err.message.startsWith(prefix)) return null;
const parsed = JSON.parse(err.message.slice(prefix.length));
```

The `"GraphQL errors: "` prefix is the SSR shape that `gqlFetch` (`frontend/src/lib/apollo/server.ts`) constructs when the GraphQL response carries `errors` and `data == null`. **Apollo Client at runtime does not produce that string.** When a `useMutation` rejection or an in-browser `client.query()` failure surfaces as a `CombinedGraphQLErrors` (or any Apollo error subclass), the typed errors live on `err.graphQLErrors: GraphQLError[]` and the message is the human-readable Apollo aggregate, not the JSON-stringified array. Passing such an error to `isUnauthenticatedGraphQLError` returns `false` — silently — and the typed-error branch never fires.

This divergence is non-obvious because the helper names omit the transport assumption. RSC pages, route handlers, and any code path that consumes `gqlFetch` use the helpers correctly. Client components that consume `useMutation` / `useQuery` results must read `err.graphQLErrors[*].extensions.code` directly.

**How to apply:** in a client component that needs typed-error discrimination after a mutation, lift the extension codes off `err.graphQLErrors` instead of calling the SSR helpers:

```ts
function liftGraphQLCodes(err: unknown): string[] {
  if (err == null || typeof err !== "object") return [];
  const maybe = (err as { graphQLErrors?: unknown }).graphQLErrors;
  if (!Array.isArray(maybe)) return [];
  const codes: string[] = [];
  for (const entry of maybe) {
    const code = (entry as { extensions?: { code?: unknown } } | null | undefined)
      ?.extensions?.code;
    if (typeof code === "string") codes.push(code);
  }
  return codes;
}

const result = await mutate({ variables }).catch((err) => {
  const codes = liftGraphQLCodes(err);
  if (codes.includes("UNAUTHENTICATED")) { setAuthBanner("expired"); return null; }
  if (codes.includes("FORBIDDEN"))       { setAuthBanner("forbidden"); return null; }
  // safe to log: codes is a fixed enum; err.message may echo user input.
  console.warn("[scope] mutation rejected", { codes });
  return null;
});
```

The `codes` payload is safe to log; the raw `err.message` may carry user-supplied content echoed by the backend (see [`redact-err-message-from-console-payloads.md`](redact-err-message-from-console-payloads.md)) and must stay out of the structured payload.

Reference: the `liftGraphQLCodes` helper in `frontend/src/app/admin/roles/[id]/edit/edit-role-client.tsx` is the canonical worked example. Its top-of-file comment ("We deliberately do not reuse `parseGqlErrors` here") exists specifically to flag this divergence for the next reader.

Related: [`Use CombinedGraphQLErrors.is(err)`](combinedgraphqlerrors-is-over-instanceof.md) covers a different concern — realm-safe detection of `CombinedGraphQLErrors`. This rule is about extension-code extraction, which works regardless of whether the runtime error is a `CombinedGraphQLErrors` or any other Error whose shape exposes `graphQLErrors`.
