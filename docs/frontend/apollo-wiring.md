# Apollo wiring

> Part of [`frontend/CLAUDE.md`](../../frontend/CLAUDE.md). See the index for related chapters.

### RSC (`gqlFetch`)

File: `frontend/src/lib/apollo/server.ts`.

Exports `gqlFetch(doc, { variables?, revalidate? })` — a plain `fetch` POST to `${env.BACKEND_URL}/query`. It serializes the document via `print(doc)` from the `graphql` package and returns typed data.

**Why not Apollo's RSC mode**: Next 16's `fetch` already handles dedup, revalidation, and caching. Adding Apollo's normalization layer on top would double-cache. `gqlFetch` stays thin.

**Why `env.BACKEND_URL` directly (not `/api/graphql`)**: RSC runs server-side and does not pass through Next rewrites — see Gotchas below.

### Browser (`ApolloNextAppProvider`)

Files:
- `frontend/src/lib/apollo/client.ts` — the `makeClient` factory.
- `frontend/src/app/providers.tsx` — wraps the app in `ApolloNextAppProvider`.

Browser code calls `/api/graphql` (same-origin via the Next rewrite — avoids CORS/cookie issues). `ApolloClient` and `InMemoryCache` come from `@apollo/client-integration-nextjs` (SSR-streaming-safe variants) — see Gotcha below.

### Apollo cache mutation patterns

**`update` callback fires on success only** (default `errorPolicy: "none"`). Guard the body with a presence check for forward-compatibility with `errorPolicy: "all"`:

```ts
update(cache, { data }) {
  if (!data?.createCardgroup?.cardgroup) return;
  // safe to write
}
```

**Create — prepend into a Connection query** via `readQuery → writeQuery`:

```ts
const existing = cache.readQuery({
  query: MyCardgroupsConnectionDocument,
  variables: CARDGROUPS_DEFAULT_VARS,
});
const newEdge = {
  __typename: "CardgroupEdge" as const,
  cursor: created.id,
  node: created,
};
cache.writeQuery({
  query: MyCardgroupsConnectionDocument,
  variables: CARDGROUPS_DEFAULT_VARS,
  data: {
    myCardgroupsConnection: existing
      ? {
          ...existing.myCardgroupsConnection,
          edges: [newEdge, ...existing.myCardgroupsConnection.edges],
          totalCount: existing.myCardgroupsConnection.totalCount + 1,
        }
      : {
          // Cold cache: build a minimal connection so the listing page renders
          // the new edge immediately when the user lands there.
          __typename: "CardgroupConnection" as const,
          edges: [newEdge],
          pageInfo: {
            __typename: "PageInfo" as const,
            hasNextPage: false,
            hasPreviousPage: false,
            startCursor: created.id,
            endCursor: created.id,
          },
          totalCount: 1,
        },
  },
});
```

**Delete — evict the entity then collect garbage** (flat-list shape only):

```ts
cache.evict({ id: cache.identify({ __typename: "Cardgroup", id: cardgroupId }) });
cache.gc();
```

For Connection types (`*Connection` / `*Edge`), use `readQuery + writeQuery` (not `cache.modify`) and align the variables shape between SSR seed and client. See `.claude/rules/pagination.md` and `docs/pagination/` for Connection create, delete, and update cache patterns.

### Pagination patterns

The reference implementation is `frontend/src/app/cardgroups/[id]/cards/use-cards-connection.ts` (consumed by `cards-client.tsx`) — it owns `useQuery` + `fetchMore`, the IntersectionObserver sentinel, the `useEffectEvent` observer callback, the parameterized `fetchNextPage({ hasNextPage, endCursor, searchQuery })` helper, the in-flight `fetchingRef`, and `fetchMoreError` state. See `docs/pagination/` for IntersectionObserver in-flight guards, `fetchMoreError` handling, `NetworkStatus.fetchMore` conventions, MockedProvider warn-spy patterns, and the sibling `useRef<string | null>` discriminator-keyed mount-effect mutation guard (used by `LearnClient` to fire `setLastViewedCardgroup` exactly once per cardgroup, not once per render).

### Bulk delete cache update pattern

Selection state lives on the client component as a `Set<string>`. A checkbox row toggles membership; the bulk-action bar renders only when the set is non-empty. The `deleteCards(ids)` mutation returns the backend's count of rows deleted, not the input count — foreign-owned ids are silently skipped at the backend, making the response count the single source of truth for cache updates.

Cache update pattern: read the Connection query via `cache.readQuery` → filter `edges` to remove the deleted ids → decrement `totalCount` by the backend's reported count (NOT `ids.length`) → `cache.writeQuery` to persist the modified Connection → `cache.evict` per id to clear normalized entries → `cache.gc()` to garbage-collect orphaned references. This mirrors the single-delete pattern; do not use `cache.modify` alone because cold caches no-op silently.

Early-return guard: the `update` callback must begin with `if (data?.deleteCards == null) return;`. Although Apollo Client normally skips `update` on network-layer rejection, a synchronous error inside the callback body will still fire `cache.evict + cache.gc` on whatever was already processed, causing cards to visually vanish while still present server-side. The guard defends against this: if the server response is absent or null the callback exits before touching the cache, so a transient failure followed by a retry leaves the UI consistent.
