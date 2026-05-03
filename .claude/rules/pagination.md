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

### Reject mixed-direction argument combos at the usecase

The repository trusts its inputs. Without explicit usecase-layer guards, malformed combinations silently re-interpret as a forward page-1 request and the client never learns why their cursor was ignored. Reject all five bad combos with `BAD_USER_INPUT` (with `extensions.field` naming the offending argument):

| Combo | Why rejected |
|---|---|
| `after` + `before` | The two cursors disagree on direction. |
| `first` + `before` | `first` implies forward, `before` implies backward. |
| `last` + `after` | `last` implies backward, `after` implies forward. |
| `before` alone (no `last`) | A backward cursor without a backward page size is ambiguous. |
| `after` alone (no `first`) | A forward cursor without a forward page size is ambiguous. |

A request with neither cursor and neither size is the legitimate "first page, server default" case and must still be accepted.

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

### Drop `optimisticResponse` for mutations that can fail with typed GraphQL errors

`@apollo/client` v3.x rolls back optimistic writes on **network** errors but not consistently on typed GraphQL errors (`FORBIDDEN`, `BAD_USER_INPUT`, etc.). For a mutation that can plausibly return one of those — e.g. a role assignment that fails authorization, or a self-demotion blocked by a server-side guard — the optimistic write persists and the cache lies until the next mount. Two acceptable postures:

1. **Drop optimistic** for mutations that can fail typed. The user pays a single round-trip of latency, but the cache stays truthful.
2. **Manual rollback in the catch branch** — `cache.evict({ id: ... })` followed by `cache.gc()` to undo whatever the optimistic update wrote.

Posture 1 is the default; reach for posture 2 only when the perceived latency cost is measurable. Never leave a typed-error-capable mutation with `optimisticResponse` and no rollback.

### Pick one data-loading mode per component

A component that accepts both an SSR-prop variant (`{ initial: T }`) and a query-id variant (`{ id: string; query }`) ends up with `useState(props.initial ?? "")` or similar — the state initializes once before the query resolves and stays at the empty default. The two modes are not interchangeable. Decide at the call site (RSC seed vs. client-driven fetch) and keep the component single-mode. If both modes are needed at different routes, write two thin wrappers around a shared presentational component instead of branching inside.

## Frontend pagination UX

### IntersectionObserver in-flight guard via `useRef<boolean>`

The in-flight guard must live in `useRef<boolean>`, not `useState`. State updates are async — the observer can fire twice in the same animation frame and both passes read the previous `false`, double-firing `fetchMore`. A mutable ref is set synchronously, reset in `.finally`, and never schedules a re-render.

### One-shot mount-effect mutation guard via `useRef<string | null>`

The same async-state hazard appears whenever a client component fires a mutation **once per discriminator value** from a `useEffect`. React 18 Strict Mode double-mounts dev-time, and any future re-render that re-runs the effect re-fires the mutation — a `useState` "did we send it yet" flag updates async and both passes read the previous value. The fix is the same shape as the IO guard, but the ref holds the **discriminating identity** the mutation was last dispatched for, not a boolean:

```ts
const lastDispatchedRef = useRef<string | null>(null);
useEffect(() => {
  if (serverCurrentValue === incomingValue) return;     // already in sync
  if (lastDispatchedRef.current === incomingValue) return; // already dispatched this run
  lastDispatchedRef.current = incomingValue;
  client.mutate({ mutation: PersistMutation, variables: { incomingValue }, update: ... })
    .catch((err) => console.warn("[scope] persist failed", { incomingValue, err }));
}, [incomingValue, serverCurrentValue, client]);
```

Two non-obvious points: (1) the SSR-seeded "current server value" lets the effect skip the network entirely when nothing changed — keep that prop, do not collapse the check to "always fire once"; (2) the mutation should run via the imperative `client.mutate(...)` (not `useMutation`) so the cache update runs regardless of caller render state and the effect's dependency surface stays narrow. Reference: `frontend/src/app/learn/[cardgroupId]/learn-client.tsx` persisting `lastViewedCardgroup`.

The "drop `optimisticResponse`" rule below applies here too: a mutation that can plausibly return `BAD_USER_INPUT` (e.g. cardgroup deleted between page render and the effect firing) should not carry an optimistic write.

### `fetchMoreError != null` halts the IO loop

Without an error halt gate, the IO keeps firing on the same failed cursor and loops invisibly with only `console.error` noise. Set state to a banner string in the `fetchMore` `.catch`, render a Retry button that clears the state and re-invokes the request, and short-circuit the IO `useEffect` while the error is set.

### `NetworkStatus.fetchMore`, not magic number

Always import the named `NetworkStatus` enum from `@apollo/client`. Magic numbers silently rot if Apollo renumbers (vanishingly rare, but the named import costs nothing).

### Capture `console.warn` for MockedProvider leaks, then assert in teardown

The assertion `nextPageCalls === 1` is partially tautological — `MockedProvider` matches per-entry, single-use, keyed on `(query, variables)`, so a leaked second `fetchMore` does not throw. It prints `"No more mocked responses for the query"` to `console.warn` and the `useQuery` hook resolves with `undefined` `data`; tests that depend on the second-page data thus silently pass on stale or missing data. The contract is **capture-then-explicit-assert**, not auto-fail-on-warn: `installApolloMockLeakSpy({ operationNames })` in `frontend/__tests__/utils/mock-apollo-paginated.ts` records every matching warning, and a `assertNoLeaks()` call in `afterEach` converts the captured set into a hard test failure. Restore the spy in the same `afterEach`. Without this, double-fetch regressions pass the call-count assertion silently.

#### Spy stacking: install order is outer-first, teardown is LIFO

When a test file installs the leak spy AND a second `vi.spyOn(console, "warn")` (e.g. to assert that a non-Apollo `console.warn("[scope] ...")` call also fires), the second `spyOn` becomes the **outer** spy: every `console.warn(...)` call hits it first, and only forwards to the leak spy if the outer spy's `mockImplementation` does so. Two consequences:

1. **Do not call `outer.mockImplementation(() => {})` on the outer spy** — it swallows every warning before the leak spy records it, and `assertNoLeaks()` becomes a no-op while real leaks ship to production unnoticed. The `cards-new-client.test.tsx` regression that exposed this rule: a `mockImplementation(() => {})` was added to silence persist-failure warnings in a single test, and the leak spy stopped catching unmatched mocks for every test in the file.
2. **Restore in LIFO order**: outer spy first (so `console.warn` is back to the leak spy's mock), then the leak spy (so `console.warn` is back to the real implementation). Reversing leaves the leak spy's mock installed permanently.

If a single test needs to suppress a specific `console.warn` call, prefer asserting it explicitly via `expect(consoleWarnSpy).toHaveBeenCalledWith(...)` — the assertion documents intent and the call still flows through to the leak spy. The leak spy itself defaults to `silent: true` (`installApolloMockLeakSpy` swallows the formatted leak warning to keep CI output clean), so the outer spy does not need its own silencer.

### Provide two `MockedResponse` entries to test a Retry-after-error path

`MockedProvider` serves entries in order, single-use. A test that mounts with only one `{ request, error }` mock can assert that the error UI appears, but clicking Retry fires `refetch()` — which consumes a second entry. With only one entry, `MockedProvider` prints `"No more mocked responses for the query"` to `console.warn` and the `useQuery` hook resolves with `undefined` data; the refetch path is never exercised in a way that can assert the success state. Supply two matching entries:

```ts
const retryMocks: MockedResponse[] = [
  { request: { query: MyQuery }, error: new Error("network failure") },
  { request: { query: MyQuery }, result: { data: { ... } } },
];
```

The first entry drives the initial error state; the second entry is consumed by `refetch()`. A single-entry test for the same scenario passes tautologically — the leak spy records the unmatched warn and `assertNoLeaks()` in `afterEach` converts it to a hard failure. Reference: `frontend/src/components/cardgroups/cardgroup-picker-sheet.test.tsx` tests S12 and S13.
