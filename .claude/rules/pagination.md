# Pagination

> Single source of truth for the Relay-style Connection design used by the cards-by-cardgroup query and the Apollo client patterns that consume it. Cross-referenced from `docs/backend.md` and `docs/frontend.md`.

## Why Relay Connection (not offset/limit)

Relay-style Connection pagination (cursor-based) is chosen because:

- **Stable across mutations.** Offset pagination shifts rows when items are inserted or deleted mid-list, causing skips or duplicates for the client. A cursor anchors to a specific row.
- **Cursor opaqueness.** Clients treat `cursor: ID!` as an opaque handle and pass it back unchanged. The server may change encoding without breaking clients.
- **Server-side determinism.** The server controls page boundaries, order, and tie-breaking, so clients cannot craft parameters that bypass ordering invariants.

## Schema-side conventions

### Use Connection over flat list for paginated queries

Paginated lists use Relay-style Connection types, not bare `[T!]!`. New paginated queries declare `<Type>Connection { edges, pageInfo, totalCount }` + `<Type>Edge { cursor, node }` and accept `(first, after, last, before, orderBy, orderDirection)`. The flat `[T!]!` shape is reserved for fixed small collections where pagination is meaningless.

### Migration via `@deprecated`

When migrating an existing flat list to a Connection type, keep the old field with `@deprecated(reason: "Use <newField>")` until all clients have moved over. Removing the old field in the same change breaks any unmigrated consumer.

## Server-side design

- **Tuple `(orderField, id)` comparison.** Ties on the order field would skip or duplicate rows. The repository emits `field op ? OR (field = ? AND id op ?)` rather than the Postgres-only `(a, b) > (?, ?)` row-constructor — the expanded form is dialect-portable. When `orderBy` is already `ID`, only `ORDER BY id` is emitted; otherwise `, id <dir>` is appended so the ordering is always total.
- **`+1` fetch trick for `hasNextPage`.** The usecase asks the repository for `first+1` rows. If the repository returns more than `first`, set `hasNextPage = true` and trim the trailing row. No second query is needed.
- **Page-size cap asymmetry (`maxPageSize` vs `pageCap`).** `maxPageSize = 100` is the user-facing cap enforced by the usecase; `pageCap = maxPageSize + 1` (101) is the repository-level limit that lets the `+1` trick survive a request at the documented maximum. Changing one without the other silently caps a layer below spec.
- **Backward pagination via direction-flip + reverse.** Natural SQL "give me N rows before X" is awkward. The repository inverts the `ORDER BY` direction, applies `LIMIT N+1`, then reverses the returned slice in memory. The usecase mirrors the trim logic on the leading edge so the page boundary stays at the tail.
- **`totalCount` via separate `COUNT(*)`.** `totalCount` is a separate `COUNT(*)` query scoped by `cardgroup_id` — extra DB round-trips for SQL simplicity, acceptable for `<= 10k` cards per group; revisit with a windowed estimate or denormalised counter if the cap grows. Run `COUNT` **before** the `first == 0 && last == 0` short-circuit so callers asking only for `totalCount` still get a real value.
- **Three layers of enums kept in sync.** `model.CardOrderBy` (gqlgen schema strings), `usecase.CardOrderBy` (typed enum, same string values), `repository.CardOrderBy` (snake_case column names). The resolver translates model → usecase; the usecase translates usecase → repository. Each layer stays free of dependencies on the others — duplication is intentional.
- **`cursorFieldValue` errors on missing column.** A silent zero-value fallback (`time.Time{}`) would generate a wrong-but-valid SQL predicate and quietly skip rows. The usecase hydrates the column required by the active `orderBy` before calling the repository; a missing column is a caller bug — surface it as `gqlerr.Internal`.

### Further reading (on-demand)

- [Cursor encoding](../../docs/pagination/cursor-encoding.md)
- [Cursor cross-aggregate validation → `BAD_USER_INPUT`](../../docs/pagination/cursor-cross-aggregate-validation.md)
- [Reject mixed-direction argument combos at the usecase](../../docs/pagination/reject-mixed-direction-combos.md)

## Frontend cache patterns

- **`cache.modify` skips non-existent fields — use `readQuery + writeQuery`.** `cache.modify` skips a field that does not yet exist, so a "build a fresh connection if `existing == null`" branch under `cache.modify` is dead code; cold-cache cases (e.g. direct landing without SSR seed) silently lose the new edge. `readQuery` returns `null` for a cold cache and `writeQuery` writes either branch unconditionally — that is the correct seam for Connection updates.
- **Connection create.** Write the new entity via `cache.writeFragment` first so any other cached edge that references the same `id` resolves correctly, then use `readQuery + writeQuery` to append a `{ cursor, node }` edge and bump `totalCount`. On a cold cache, build a minimal `<Type>Connection` with `pageInfo.hasNextPage = false` so the IO loop stays idle.
- **Connection delete.** Filter the edge out of the cached connection, decrement `totalCount` (clamped at 0 — the deleted item may live on a page that was never fetched into edges), then `cache.evict` + `cache.gc()` to drop the normalised entity. Eviction alone is insufficient — the connection field is a list of `{ cursor, node }` objects whose `node` reference is broken by evict but the parent edge stays in the array.
- **Connection update (cache normalization).** Rely on Apollo cache normalization (entities with `id` are normalized by default). No manual `update` callback is needed; mutations that touch a normalised entity propagate to every cached query that reads it.
- **Drop `optimisticResponse` for mutations that can fail with typed GraphQL errors.** `@apollo/client` v3.x rolls back optimistic writes on **network** errors but not consistently on typed GraphQL errors (`FORBIDDEN`, `BAD_USER_INPUT`, etc.). For a mutation that can plausibly return one of those — e.g. a role assignment that fails authorization, or a self-demotion blocked by a server-side guard — the optimistic write persists and the cache lies until the next mount. Default posture: drop `optimisticResponse` and pay one round-trip of latency. Alternative when latency is measurable: manual rollback in the catch branch via `cache.evict({ id: ... })` + `cache.gc()`. Never leave a typed-error-capable mutation with `optimisticResponse` and no rollback.
- **Pick one data-loading mode per component.** A component that accepts both an SSR-prop variant (`{ initial: T }`) and a query-id variant (`{ id: string; query }`) ends up with `useState(props.initial ?? "")` or similar — the state initializes once before the query resolves and stays at the empty default. The two modes are not interchangeable. Decide at the call site (RSC seed vs. client-driven fetch) and keep the component single-mode. If both modes are needed at different routes, write two thin wrappers around a shared presentational component instead of branching inside.

### Further reading (on-demand)

- [Variables shape MUST match between SSR seed and client cache reads](../../docs/pagination/variables-shape-must-match.md)
- [Migrating a flat list to a Connection: write to BOTH cached shapes during the deprecation window](../../docs/pagination/migrating-flat-list-to-connection.md)
- [Optimistic-rollback cache key MUST track the active query variables, not the default factory](../../docs/pagination/optimistic-rollback-cache-key.md)
- [Do not reuse one mutation's Apollo-managed error state for a sibling mutation's failure surface](../../docs/pagination/do-not-reuse-mutation-error-state.md)

## Frontend pagination UX

- **`fetchMoreError != null` halts the IO loop.** Without an error halt gate, the IO keeps firing on the same failed cursor and loops invisibly with only `console.error` noise. Set state to a banner string in the `fetchMore` `.catch`, render a Retry button that clears the state and re-invokes the request, and short-circuit the IO `useEffect` while the error is set.
- **`NetworkStatus.fetchMore`, not magic number.** Always import the named `NetworkStatus` enum from `@apollo/client`. Magic numbers silently rot if Apollo renumbers (vanishingly rare, but the named import costs nothing).

### Further reading (on-demand)

- [IntersectionObserver in-flight guard via `useRef<boolean>`](../../docs/pagination/intersection-observer-in-flight-guard.md)
- [One-shot mount-effect mutation guard via `useRef<string | null>`](../../docs/pagination/one-shot-mount-effect-mutation-guard.md)
- [`useEffect` cleanup must not fire `void asyncFn()` for navigation-time side effects](../../docs/pagination/useeffect-cleanup-no-void-async.md)
- [`beforeunload` flush is browser-cancellable; gate the warn on pending count](../../docs/pagination/beforeunload-flush-gate-on-pending.md)
- [Capture `console.warn` for MockedProvider leaks, then assert in teardown](../../docs/pagination/capture-mockedprovider-warn-leaks.md)
- [Stabilise `requestNextPage` via the cursor / search / hasNextPage ref triplet](../../docs/pagination/stabilise-request-next-page-ref-triplet.md)
- [Synchronous SSR cache seed in the render body, not in a `useEffect`](../../docs/pagination/synchronous-ssr-cache-seed.md)
- [Split debounce from immediate-reset effects on the same input](../../docs/pagination/split-debounce-from-immediate-reset.md)
- [Load-bearing `biome-ignore` comments](../../docs/pagination/load-bearing-biome-ignore.md)
- [Provide two `MockedResponse` entries to test a Retry-after-error path](../../docs/pagination/two-mocked-responses-for-retry-test.md)
