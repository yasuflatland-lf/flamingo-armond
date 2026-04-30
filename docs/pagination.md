# Pagination

> Single source of truth for the Relay-style Connection design used by the cards-by-cardgroup query and the Apollo client patterns that consume it. Cross-referenced from `docs/backend.md` and `docs/frontend.md`.

## Why Relay Connection (not offset/limit)

Relay-style Connection pagination (cursor-based) is chosen because:

- **Stable across mutations.** Offset pagination shifts rows when items are inserted or deleted mid-list, causing skips or duplicates for the client. A cursor anchors to a specific row.
- **Cursor opaqueness.** Clients treat `cursor: ID!` as an opaque handle and pass it back unchanged. The server may change encoding without breaking clients.
- **Server-side determinism.** The server controls page boundaries, order, and tie-breaking, so clients cannot craft parameters that bypass ordering invariants.

## Server-side design

### Cursor encoding

The cursor is the bare entity UUID — not base64-encoded. UUID is already opaque; double-encoding adds no security and forces clients to decode before comparing. The schema declares `cursor: ID!` and the resolver assigns the entity ID directly into edges.

### Tuple `(orderField, id)` comparison

Ties on the order field would otherwise skip or duplicate rows. The repository emits SQL of the form `field op ? OR (field = ? AND id op ?)` rather than the Postgres-only `(a, b) > (?, ?)` row-constructor — the expanded form is portable across SQL dialects. When the user-supplied `orderBy` is already `ID`, only `ORDER BY id` is emitted; otherwise `, id <dir>` is appended so the ordering is always total.

### `+1` fetch trick for `hasNextPage`

The usecase asks the repository for `first+1` rows. If the repository returns more than `first`, set `hasNextPage = true` and trim the trailing row. No second query is needed.

### Page-size cap asymmetry (`maxPageSize` vs `pageCap`)

Two constants work together:

- `maxPageSize = 100` — user-facing cap enforced by the usecase.
- `pageCap = maxPageSize + 1` (i.e. 101) — repository-level limit that lets the `+1` trick survive a request at the documented maximum.

Document both constants. Changing one without the other silently caps a layer below spec.

### Backward pagination via direction-flip + reverse

Natural SQL "give me N rows before X" is awkward. The repository inverts the `ORDER BY` direction, applies `LIMIT N+1`, then reverses the returned slice in memory. The usecase mirrors the trim logic on the leading edge so the page boundary stays at the tail.

### `totalCount` via separate `COUNT(*)`

`totalCount` is a separate `COUNT(*)` query scoped by `cardgroup_id`. This trades extra DB round-trips for SQL simplicity — acceptable for `<= 10k` cards per group; revisit with a windowed estimate or a denormalised counter if the cap grows. Run `COUNT` **before** the `first == 0 && last == 0` short-circuit so callers asking only for `totalCount` still get a real value.

### Cursor cross-aggregate validation → `BAD_USER_INPUT`

A cursor pointing at a card in another cardgroup is treated as a malformed user-supplied parameter. Returning `UNAUTHENTICATED` would leak existence of cards in other cardgroups; `BAD_USER_INPUT` with `field = "after"` / `field = "before"` is the correct posture.

### Three layers of enums kept in sync

- `model.CardOrderBy` — gqlgen-generated, schema strings.
- `usecase.CardOrderBy` — typed enum local to the usecase, same string values.
- `repository.CardOrderBy` — snake_case column names (e.g. `created_at`).

The resolver translates model → usecase; the usecase translates usecase → repository. Each layer stays free of dependencies on the others — no GraphQL types in the repository, no `gorm` types in the resolver. The duplication is intentional.

### `cursorFieldValue` errors on missing column

A silent zero-value fallback (e.g. `time.Time{}`) would generate a wrong-but-valid SQL predicate and quietly skip rows. The usecase hydrates the column required by the active `orderBy` before calling the repository, so a missing column is a caller bug — surface it as `gqlerr.Internal` rather than return wrong rows.

## Schema-side conventions

### Use Connection over flat list for paginated queries

Paginated lists use Relay-style Connection types, not bare `[T!]!`. New paginated queries declare `<Type>Connection { edges, pageInfo, totalCount }` + `<Type>Edge { cursor, node }` and accept `(first, after, last, before, orderBy, orderDirection)`. The flat `[T!]!` shape is reserved for fixed small collections where pagination is meaningless.

### Migration via `@deprecated`

When migrating an existing flat list to a Connection type, keep the old field with `@deprecated(reason: "Use <newField>")` until all clients have moved over. Removing the old field in the same change breaks any unmigrated consumer.

## Frontend cache patterns

### `cache.modify` skips non-existent fields — use `readQuery + writeQuery`

`cache.modify` skips a field that does not yet exist in the cache, so a "build a fresh connection if `existing == null`" branch under `cache.modify` is dead code. Cold-cache cases (e.g. user lands directly on the page without an SSR seed) silently lose the new edge. `readQuery` returns `null` for a cold cache and `writeQuery` writes either branch unconditionally — that is the correct seam for Connection updates.

### Variables shape MUST match between SSR seed and client cache reads

Apollo's cache key is built from canonical-stringified variables. Hard-coding `first: 20` in `page.tsx` while the client uses a `PAGE_SIZE` constant is coincidence-only; bumping the constant breaks the seed-then-update pipeline silently. Lift shared connection variables to one module (e.g. `cards/queries.ts` exports `CARDS_PAGE_SIZE`) that both SSR and client import.

### Connection create

Write the new entity via `cache.writeFragment` first so any other cached edge that references the same `id` resolves correctly, then use `readQuery + writeQuery` to append a `{ cursor, node }` edge and bump `totalCount`. On a cold cache, build a minimal `<Type>Connection` with `pageInfo.hasNextPage = false` so the IO loop stays idle.

### Connection delete

Filter the edge out of the cached connection, decrement `totalCount` (clamped at 0 — the deleted item may live on a page that was never fetched into edges), then `cache.evict` + `cache.gc()` to drop the normalised entity. Eviction alone is not enough — the connection field is a list of `{ cursor, node }` objects whose `node` reference is broken by evict but the parent edge stays in the array.

### Connection update (cache normalization)

Rely on Apollo cache normalization (entities with `id` are normalized by default). No manual `update` callback is needed; mutations that touch a normalised entity propagate to every cached query that reads it.

## Frontend pagination UX

### IntersectionObserver in-flight guard via `useRef<boolean>`

The in-flight guard must live in `useRef<boolean>`, not `useState`. State updates are async — the observer can fire twice in the same animation frame and both passes read the previous `false`, double-firing `fetchMore`. A mutable ref is set synchronously, reset in `.finally`, and never schedules a re-render.

### `fetchMoreError != null` halts the IO loop

Without an error halt gate, the IO keeps firing on the same failed cursor and loops invisibly with only `console.error` noise. Set state to a banner string in the `fetchMore` `.catch`, render a Retry button that clears the state and re-invokes the request, and short-circuit the IO `useEffect` while the error is set.

### `NetworkStatus.fetchMore`, not magic number

Always import the named `NetworkStatus` enum from `@apollo/client`. Magic numbers silently rot if Apollo renumbers (vanishingly rare, but the named import costs nothing).

### Spy on `console.warn` for MockedProvider leaks

The assertion `nextPageCalls === 1` is partially tautological — `MockedProvider` only consumes a mock once, so a leaked second `fetchMore` produces a `"No more mocked responses for the query"` warning rather than an extra invocation. Spy on `console.warn` and assert it does not see that string for the query of interest; restore the spy in `afterEach`. Without this, double-fetch regressions pass the call-count assertion silently.
