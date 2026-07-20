# Migrating a list screen onto the shared `useConnectionPagination` hook

All five cursor-paginated list screens — cards, cardgroups, catalog, admin
masters, admin users — consume the single `useConnectionPagination` hook
(`frontend/src/lib/pagination/use-connection-pagination.ts`). The hook owns the
Apollo `useQuery` + `fetchMore` + `IntersectionObserver` loop and the three
`.claude/rules/pagination.md` invariants. When migrating a screen's inline IO
onto it, four traps recur.

## Why

The hook is the **sole owner** of the underlying `useQuery`. Everything the
screen previously read off its own `useQuery` result — `data` / `edges` /
`pageInfo` / `loading` / `networkStatus` / `error` / `refetch` — must now come
from the hook's return. A screen cannot keep a second `useQuery` for the same
document just to recover one of those fields without double-subscribing and
splitting the cache.

## What

### 1. Source `refetch` from the hook — never re-add a parallel `useQuery`

A screen that renders a query-error banner with a **Retry**, or a
mutation-conflict reload (admin users' `reloadEditedUser` after a
`ConcurrentUpdateError`), needs `refetch`. The hook exposes it
(`refetch: useQuery.Result<TData, TVars>["refetch"]`, returned straight from the
owned `useQuery`); take it from the hook return. Re-adding a
`useQuery(SameDocument, …)` purely to obtain `refetch` double-subscribes to the
same document and can split the cache. The SSR-seeded screens (cards /
cardgroups / catalog) ignore `refetch`; only the two admin screens consume it,
which is why it was added to the hook in the admin migration, not the original
extraction.

### 2. Keep the screen's OTHER query hooks

Migrating the **main** list query does not mean deleting the `useQuery` /
`useLazyQuery` import wholesale. Admin users keeps a `useQuery(AdminRolesQuery)`
and a `useLazyQuery(AdminUserQuery)` (the edit-user loading gate); only its
primary connection query moves into the hook. Dropping the import breaks the
sibling queries the screen still owns.

### 3. Preserve the exact cache-key variables

Build the hook's `variables` from the screen's existing base-vars object via
`useMemo`, including aggregate-specific fields (`orderBy: "SORT_ORDER"`,
`orderDirection: "ASC"` for masters; bare `{ first, search }` for users). A
re-spelled key splits the cache from the screen's own mutation cache writes —
e.g. the masters create handler reads/writes the `search: null` variant, so the
list query's key must match it exactly. See
[`variables-shape-must-match.md`](variables-shape-must-match.md).

### 4. Move the screen's entry in the regression guard

`frontend/src/app/pagination-ref-triplet-removal-rule.test.ts` is the
enforcement. A migrated screen moves from `ioOwningSites` (asserts
`useEffectEvent` is present and the `endCursorRef` / `hasNextPageRef` /
`searchQueryRef` triplet is absent) to `migratedSites` (asserts
`useConnectionPagination` is present and `new IntersectionObserver` /
`useEffectEvent` are absent). After all five screens migrate, the hook itself is
the only remaining `ioOwningSites` entry.

Register the file that actually carries the `useConnectionPagination` literal,
not a caller one level up. The three cards screens (cardgroup cards, admin
master cards, catalog deck cards) reach the hook through the shared
`useEntityCardsConnection` factory (`src/lib/pagination/use-entity-cards-connection.ts`),
so that factory — not the per-entity `use-*-cards-connection.ts` wrappers or
their clients — is the `migratedSites` entry. See
[`.claude/rules/pagination.md`](../../.claude/rules/pagination.md) § "Frontend
pagination UX".

## How to apply

1. Replace the inline IO — `fetchingRef`, `fetchNextPage`,
   `requestNextPageFromObserver`, the observer `useEffect`, the immediate-reset
   effect, and the local `fetchMoreError` state — with one
   `useConnectionPagination(...)` call. Inject `document`, memoized `variables`,
   `searchQuery`, `selectConnection`, `buildFetchMoreVariables`,
   `mergeConnection`, `initial`, `resolveFetchMoreError`, and `logScope`. Build
   `mergeConnection` with the shared
   `makeMergeConnection<TData>(connectionField)` factory
   (`src/lib/pagination/make-merge-connection.ts`) rather than hand-rolling the
   edges concat, and hold the instance at module scope so its identity stays
   stable — the hook takes it as a `useCallback` dependency.
2. Wire the screen's Retry buttons to the hook's `retryFetchMore` (the fetchMore
   banner) and `refetch` (the initial-query-error banner). See
   [`hook-expose-actions-not-setters.md`](../frontend/typescript-conventions/hook-expose-actions-not-setters.md).
3. Keep the screen-specific concerns in place: mutation handlers + cache writes,
   the delete `cache.modify` filter, the edit-sheet lazy query, and the
   conflict-banner reload
   ([`preserve-banner-across-same-entity-refetch.md`](../frontend/typescript-conventions/preserve-banner-across-same-entity-refetch.md)).
4. Post-flight grep — must be empty (the hook is the only owner, and it is a
   `.ts` file):

   ```bash
   grep -rln "requestNextPageFromObserver\|new IntersectionObserver" \
     frontend/src/app --include='*.tsx' | grep -v test
   ```
