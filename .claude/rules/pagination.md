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

## Server-side design (on-demand)

- [Cursor encoding](../../docs/pagination/cursor-encoding.md)
- [Tuple `(orderField, id)` comparison](../../docs/pagination/tuple-order-id-comparison.md)
- [`+1` fetch trick for `hasNextPage`](../../docs/pagination/plus-one-fetch-trick.md)
- [Page-size cap asymmetry (`maxPageSize` vs `pageCap`)](../../docs/pagination/page-size-cap-asymmetry.md)
- [Backward pagination via direction-flip + reverse](../../docs/pagination/backward-pagination-direction-flip.md)
- [`totalCount` via separate `COUNT(*)`](../../docs/pagination/totalcount-via-separate-count.md)
- [Cursor cross-aggregate validation → `BAD_USER_INPUT`](../../docs/pagination/cursor-cross-aggregate-validation.md)
- [Reject mixed-direction argument combos at the usecase](../../docs/pagination/reject-mixed-direction-combos.md)
- [Three layers of enums kept in sync](../../docs/pagination/three-layers-of-enums.md)
- [`cursorFieldValue` errors on missing column](../../docs/pagination/cursor-field-value-errors-on-missing-column.md)

## Frontend cache patterns (on-demand)

- [`cache.modify` skips non-existent fields — use `readQuery + writeQuery`](../../docs/pagination/cache-modify-skips-nonexistent-fields.md)
- [Variables shape MUST match between SSR seed and client cache reads](../../docs/pagination/variables-shape-must-match.md)
- [Migrating a flat list to a Connection: write to BOTH cached shapes during the deprecation window](../../docs/pagination/migrating-flat-list-to-connection.md)
- [Connection create](../../docs/pagination/connection-create.md)
- [Connection delete](../../docs/pagination/connection-delete.md)
- [Optimistic-rollback cache key MUST track the active query variables, not the default factory](../../docs/pagination/optimistic-rollback-cache-key.md)
- [Connection update (cache normalization)](../../docs/pagination/connection-update.md)
- [Do not reuse one mutation's Apollo-managed error state for a sibling mutation's failure surface](../../docs/pagination/do-not-reuse-mutation-error-state.md)
- [Drop `optimisticResponse` for mutations that can fail with typed GraphQL errors](../../docs/pagination/drop-optimistic-response-typed-errors.md)
- [Pick one data-loading mode per component](../../docs/pagination/pick-one-data-loading-mode.md)

## Frontend pagination UX (on-demand)

- [IntersectionObserver in-flight guard via `useRef<boolean>`](../../docs/pagination/intersection-observer-in-flight-guard.md)
- [One-shot mount-effect mutation guard via `useRef<string | null>`](../../docs/pagination/one-shot-mount-effect-mutation-guard.md)
- [`useEffect` cleanup must not fire `void asyncFn()` for navigation-time side effects](../../docs/pagination/useeffect-cleanup-no-void-async.md)
- [`beforeunload` flush is browser-cancellable; gate the warn on pending count](../../docs/pagination/beforeunload-flush-gate-on-pending.md)
- [`fetchMoreError != null` halts the IO loop](../../docs/pagination/fetchmore-error-halts-io-loop.md)
- [`NetworkStatus.fetchMore`, not magic number](../../docs/pagination/networkstatus-fetchmore-named-import.md)
- [Capture `console.warn` for MockedProvider leaks, then assert in teardown](../../docs/pagination/capture-mockedprovider-warn-leaks.md)
- [Stabilise `requestNextPage` via the cursor / search / hasNextPage ref triplet](../../docs/pagination/stabilise-request-next-page-ref-triplet.md)
- [Synchronous SSR cache seed in the render body, not in a `useEffect`](../../docs/pagination/synchronous-ssr-cache-seed.md)
- [Split debounce from immediate-reset effects on the same input](../../docs/pagination/split-debounce-from-immediate-reset.md)
- [Load-bearing `biome-ignore` comments](../../docs/pagination/load-bearing-biome-ignore.md)
- [Provide two `MockedResponse` entries to test a Retry-after-error path](../../docs/pagination/two-mocked-responses-for-retry-test.md)
