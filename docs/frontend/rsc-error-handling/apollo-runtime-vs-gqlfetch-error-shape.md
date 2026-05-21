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

**How to apply:** in a client component that needs typed-error discrimination after a mutation, use the shared `liftGraphQLCodes` helper from `@/lib/apollo/graphql-errors`. The helper narrows via `CombinedGraphQLErrors.is(err)` and iterates `err.errors` — both are Apollo Client v4 names. The v3 shape (`err.graphQLErrors`) is gone from the public API; readers built on the v3 field name will return `[]` for every real client-runtime error and the typed-error branch will silently never fire:

```ts
import { liftGraphQLCodes } from "@/lib/apollo/graphql-errors";

const result = await mutate({ variables }).catch((err) => {
  const codes = liftGraphQLCodes(err);
  if (codes.includes("UNAUTHENTICATED")) { setAuthBanner("expired"); return null; }
  if (codes.includes("FORBIDDEN"))       { setAuthBanner("forbidden"); return null; }
  // safe to log: codes is a fixed enum; err.message may echo user input.
  console.warn("[scope] mutation rejected", { codes });
  return null;
});
```

The implementation in `frontend/src/lib/apollo/graphql-errors.ts`:

```ts
import { CombinedGraphQLErrors } from "@apollo/client/errors";

export function liftGraphQLCodes(err: unknown): string[] {
  if (!CombinedGraphQLErrors.is(err)) return [];
  const codes: string[] = [];
  for (const entry of err.errors) {
    const code = entry?.extensions?.code;
    if (typeof code === "string") codes.push(code);
  }
  return codes;
}
```

Two things to keep stable: (1) the guard uses `CombinedGraphQLErrors.is(err)` rather than `instanceof` — the latter fails across module-realm boundaries (see [`combinedgraphqlerrors-is-over-instanceof.md`](combinedgraphqlerrors-is-over-instanceof.md)); (2) the iteration reads `err.errors`, the v4 field name. Plain-object test fixtures shaped like `{ graphQLErrors: [...] }` pass through `liftGraphQLCodes` returning `[]` and look like a working test, while production catches at runtime reject `CombinedGraphQLErrors.is(err) === false` and the dispatch UX dies silently. Test fixtures for this helper MUST construct a real `new CombinedGraphQLErrors([...])` so the `.is()` shape check passes — never a plain object with a `graphQLErrors` key.

The `codes` payload is safe to log; the raw `err.message` may carry user-supplied content echoed by the backend (see [`redact-err-message-from-console-payloads.md`](redact-err-message-from-console-payloads.md)) and must stay out of the structured payload.

Reference: `frontend/src/lib/apollo/graphql-errors.ts` (`liftGraphQLCodes`) is the canonical implementation, consumed by `frontend/src/app/admin/roles/[id]/edit/edit-role-client.tsx`, `frontend/src/app/admin/roles/new/new-role-client.tsx`, and `frontend/src/app/admin/users/[id]/edit/admin-user-edit-client.tsx`.

Related: [`Use CombinedGraphQLErrors.is(err)`](combinedgraphqlerrors-is-over-instanceof.md) covers a different concern — realm-safe detection of `CombinedGraphQLErrors`. This rule is about extension-code extraction, which works regardless of whether the runtime error is a `CombinedGraphQLErrors` or any other Error whose shape exposes `graphQLErrors`. For a fire-and-forget mutation that needs `liftGraphQLCodes` in its `.catch` branch plus distinct warns for null payloads and non-success variants in `.then`, see [`fire-and-forget-mutation-warn-on-null-and-non-success.md`](fire-and-forget-mutation-warn-on-null-and-non-success.md).
