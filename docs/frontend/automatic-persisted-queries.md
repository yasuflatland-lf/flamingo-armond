# Automatic Persisted Queries

> Part of [`docs/frontend.md`](../frontend.md). See the index for related chapters.

Wire format and POST-only rationale live in `docs/observability.md`. Frontend-only implementation notes follow.

The browser-side Apollo Client chain is:

```ts
from([authLink, makeApqLink(), httpLink])
```

Order matters:

- `authLink` runs first so hash-only POST bodies still carry the Supabase `Authorization: Bearer` header.
- `apqLink` runs second so it can rewrite the outbound body to use `extensions.persistedQuery.sha256Hash`.
- `httpLink` is terminal.

### sha256 via native WebCrypto

`frontend/src/lib/apollo/sha256.ts` wraps `crypto.subtle.digest('SHA-256', ...)` and is shared with the APQ link. We intentionally do **not** add the `crypto-hash` npm dependency — native WebCrypto is available in every modern browser and in Node 19+ (which covers Next.js RSC).

### `useGETForHashedQueries: false`

The Apollo docs allow switching hash-only requests to GET with the query string. We deliberately keep POST because the Supabase access token travels in the `Authorization` header today; if we ever move to query-param auth, this default would leak the token into server access logs. (See `docs/observability.md` for the full rationale.)

### Apollo link chain test gotchas

**`@apollo/client-integration-nextjs` prepends two internal streaming links.** `@apollo/client-integration-nextjs` prepends two internal links (`ReadFromReadableStreamLink`, `TeeToReadableStreamLink`) to the user-supplied link chain before any user links execute. These support RSC streaming. As a result, the live chain assembled from `from([authLink, apqLink, httpLink])` has **five** segments, not three. Tests that assert `client.link.length === 3` or similar absolute counts will fail. Assert the **relative order** of the user-supplied links instead (e.g. verify that `authLink` appears before `apqLink` in the chain, not that the chain has exactly three nodes).

**`ApolloLink.from` builds a binary tree, not a flat list.** `ApolloLink.from([a, b, c])` produces a binary tree of `ApolloLink` "concat" glue nodes; `a`, `b`, `c` are the leaves. When traversing `link.left` / `link.right` to inspect the chain in tests, stop recursing when you reach a named subclass (`HttpLink`, `PersistedQueryLink`, `SetContextLink`). Descending into `HttpLink` reveals its own internal `ClientAwarenessLink` + `BaseHttpLink` pair and pollutes the segment list with internal implementation details.

