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

The same rule extends to **owner-scoped** Connection queries that have no parent aggregate (e.g. `myCardgroupsConnection`): a cursor for a cardgroup belonging to another owner must also surface as `BAD_USER_INPUT`. The non-obvious case is the **`orderBy = ID` fast path**: the usecase has nothing to hydrate (the cursor's only column IS its id), so there is a temptation to skip the cursor lookup entirely. Skipping it lets an attacker probe foreign-cardgroup existence by paging past a guessed id and observing whether any rows come back. The cursor lookup must run on the ID-orderBy branch as well, purely as an ownership gate; treat the lookup's "wrong owner" outcome the same as the "not found" outcome (`BAD_USER_INPUT` with `field = "after"` / `"before"`) so the response shape is identical for "exists but foreign" and "does not exist". Reference: `backend/internal/usecase/cardgroup.go` `resolveCardgroupCursor` runs the FindByID + ownership compare even when `orderBy == ID` and the switch arm has no column to populate.

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

**Export the full default-variables object, not just the page-size scalar.** A `PAGE_SIZE` constant alone leaves three call sites (RSC seed, client `useQuery`, mutation `update` callback) free to disagree on which other variables make it into the cache key — `{ first }` vs `{ first, search: null }` vs `{ first: 20 }` all canonicalise to different keys, and the absence of `search: null` in one branch silently splits the cache. Export a `<TYPE>_DEFAULT_VARS` object typed as the query's generated `*QueryVariables`, and require every read/write site to use it (or spread from it):

```ts
// frontend/src/app/cardgroups/queries.ts
export const CARDGROUPS_PAGE_SIZE = 20;
export const CARDGROUPS_DEFAULT_VARS: MyCardgroupsConnectionQueryVariables = {
  first: CARDGROUPS_PAGE_SIZE,
  search: null,
};
```

Three call sites consume it: `page.tsx` `gqlFetch(..., { variables: CARDGROUPS_DEFAULT_VARS })`, the client `useQuery({ variables: searchQuery === null ? CARDGROUPS_DEFAULT_VARS : { ...CARDGROUPS_DEFAULT_VARS, search: searchQuery } })`, and the create-mutation `update` callback `cache.readQuery({ ..., variables: CARDGROUPS_DEFAULT_VARS })`. The TypeScript type assertion makes any future variable added to the query schema (`orderBy`, etc.) a compile-time prompt to decide whether the new variable belongs in the default — silent additions that drift one call site away from the others surface as type errors. Reference: `frontend/src/app/cardgroups/queries.ts` (`CARDGROUPS_DEFAULT_VARS`).

### Migrating a flat list to a Connection: write to BOTH cached shapes during the deprecation window

The `@deprecated` schema migration above (§ "Migration via `@deprecated`") leaves the old flat-list query and the new Connection query coexisting in the codebase. A mutation `update` callback that creates an entity must write to BOTH cached shapes for the entire window where any consumer still reads the deprecated query — otherwise the consumer of the old query (often a sibling component like a picker sheet that has not yet migrated) shows stale data after a successful create:

```ts
update(cache, { data }) {
  if (!data?.createCardgroup?.cardgroup) return;
  const created = data.createCardgroup.cardgroup;
  // Deprecated flat list — keep updating until all readers move over.
  const flat = cache.readQuery({ query: MyCardgroupsDocument });
  cache.writeQuery({
    query: MyCardgroupsDocument,
    data: { myCardgroups: [created, ...(flat?.myCardgroups ?? [])] },
  });
  // New Connection — readQuery + writeQuery with the shared default vars.
  const conn = cache.readQuery({
    query: MyCardgroupsConnectionDocument,
    variables: CARDGROUPS_DEFAULT_VARS,
  });
  cache.writeQuery({ /* prepend edge, bump totalCount, or build cold-cache shape */ });
}
```

Removing the deprecated branch is the last step of the migration, after a grep confirms zero callers of the old document. Reference: `frontend/src/app/cardgroups/new/new-cardgroup-client.tsx`.

### Connection create

Write the new entity via `cache.writeFragment` first so any other cached edge that references the same `id` resolves correctly, then use `readQuery + writeQuery` to append a `{ cursor, node }` edge and bump `totalCount`. On a cold cache, build a minimal `<Type>Connection` with `pageInfo.hasNextPage = false` so the IO loop stays idle.

### Connection delete

Filter the edge out of the cached connection, decrement `totalCount` (clamped at 0 — the deleted item may live on a page that was never fetched into edges), then `cache.evict` + `cache.gc()` to drop the normalised entity. Eviction alone is not enough — the connection field is a list of `{ cursor, node }` objects whose `node` reference is broken by evict but the parent edge stays in the array.

### Optimistic-rollback cache key MUST track the active query variables, not the default factory

A `cardsDefaultVars(cardgroupId)` factory produces the cache vars for the unfiltered query (e.g. `{ cardgroupId, first: 20, search: null }`). When the user has an active filter (e.g. `searchQuery !== null`), the live `useQuery` is keyed on a **different** cache entry. A delete handler that snapshots and writes via the default-vars factory calls `readQuery` on the unfiltered entry, gets `null`, and the `if (snapshot)` guard silently skips the optimistic remove — the deleted row stays visible until the server commit settles. The same miss affects bulk-delete `update` callbacks.

**Why:** Apollo's cache key is the canonical-stringified variables object. The default-vars factory and the active-query variables are only equal when no filter is active; as soon as `searchQuery !== null`, they diverge and target different cache entries. This is the dual of § "Variables shape MUST match between SSR seed and client cache reads": that rule governs the three SSR-seed/client-query/update call sites all agreeing on the *default* key; this rule governs mutation `update` and optimistic-rollback callbacks agreeing with the *active* key.

**How to apply:** derive the snapshot read and rollback `writeQuery` vars from the same `queryVariables` memo that the component's active `useQuery` uses. Add `queryVariables` to the `useCallback` dep array. Reserve the default-vars factory for SSR seed and cold-cache base reads only. Reference: `frontend/src/app/cardgroups/[id]/cards/cards-client.tsx` `handleDeleteRow` and the bulk-delete `update` callback — both switched from `cardsDefaultVars(cardgroupId)` to `queryVariables`.

### Connection update (cache normalization)

Rely on Apollo cache normalization (entities with `id` are normalized by default). No manual `update` callback is needed; mutations that touch a normalised entity propagate to every cached query that reads it.

### Do not reuse one mutation's Apollo-managed error state for a sibling mutation's failure surface

`useMutation` returns a managed `error` field that clears automatically on the next call to the same mutation. If a component runs two independent mutations (e.g. `createCard` and `updateCard`), using `createError` from `useMutation(CreateCardMutation)` to display a failure message from `updateCard` causes the banner to vanish the moment the user retries `createCard` — even before the user dismisses the error. The clearing is silent; the user sees the banner disappear with no explanation.

Use a sibling `useState<string | null>` for any error that belongs to a second mutation (or any flow not managed by the primary hook):

```ts
// correct: each mutation gets its own error surface
const [createCard, { error: createError }] = useMutation(CreateCardMutation);
const [updateCard] = useMutation(UpdateCardMutation);
const [overwriteError, setOverwriteError] = useState<string | null>(null);

// in the overwrite handler:
updateCard({ ... }).catch((err) => {
  setOverwriteError(getBackendErrorBanner(err) ?? "Overwrite failed");
});
```

The `useState` error lives until the user explicitly dismisses it or the component unmounts — it does not clear on unrelated mutation interactions. Reference: `frontend/src/app/cards/new/cards-new-client.tsx` (`overwriteError` state alongside `createError`).

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

**Reset the guard ref AND the error banner when the active filter changes.** A debounced search input that drives the query's `search` variable is a second axis of "the previous in-flight cursor is now stale": between the user starting to type and the debounced `searchQuery` update, a `fetchMore` call carrying the prior page's `endCursor` may resolve into a different result-set's edge list, or fail mid-flight and leave the IO loop halted on a `fetchMoreError` banner that is no longer relevant to what the user is now searching for. The fix is a `useEffect` keyed on the active filter that clears both ref and banner state:

```ts
// when the active search query changes, drop any in-flight guard + stale error.
useEffect(() => {
  fetchingRef.current = false;
  setFetchMoreError(null);
}, [searchQuery]);
```

The dep array intentionally lists only the trigger (`searchQuery`); the body does not read it. Add a `biome-ignore lint/correctness/useExhaustiveDependencies` comment naming the trigger-not-read intent so the rule does not silently re-engage with future code-mod tools. Reference: `frontend/src/app/cardgroups/cardgroups-client.tsx`.

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

### `useEffect` cleanup must not fire `void asyncFn()` for navigation-time side effects

Putting a flush call such as `flushPendingDeletes()` inside the `useEffect` cleanup function (`return () => { void asyncFn(); }`) is unreliable. The cleanup fires synchronously on unmount or dep-change; the returned Promise is voided; React does not keep the component alive while the async work runs; in-flight mutations can be abandoned mid-flight with no error surface.

**Why:** the cleanup callback is not an async boundary — `void asyncFn()` discards the Promise before the first `await` inside it can settle. Apollo's mutation dispatch happens inside the awaited network round-trip; voiding the Promise means the network request may never be enqueued, depending on how far execution got before the component was torn down.

**How to apply:** detect the pathname change via a `previousPathnameRef = useRef(pathname)` and fire the flush in the **effect body** when `pathname` differs from the ref. The async work runs while the component is still mounted and the Apollo client context is alive:

```ts
const previousPathnameRef = useRef(pathname);
useEffect(() => {
  if (previousPathnameRef.current !== pathname) {
    void flushPendingDeletes();
    previousPathnameRef.current = pathname;
  }
}, [pathname]);
```

This pairs with the IntersectionObserver in-flight guard (§ above): both patterns use a `useRef` to track external-trigger state that must not be read asynchronously. Reference: `frontend/src/app/cardgroups/[id]/cards/cards-client.tsx` `previousPathnameRef` pattern.

### `beforeunload` flush is browser-cancellable; gate the warn on pending count

A `beforeunload` listener that unconditionally fires `void flushPendingDeletes()` produces false-positive operator noise on every routine navigation even when there is nothing pending. More critically, modern browsers cancel pending `fetch` / XHR requests when `beforeunload` fires unless the request uses `keepalive: true` or `sendBeacon` — neither of which is viable for GraphQL endpoints that require `Authorization` headers. The flush is therefore a best-effort hint, not a guarantee.

**How to apply:** gate both the warn and the flush on `_pendingCount() > 0` to avoid signal noise on routine navigation. Document the browser-cancellation limitation in a code comment so a future maintainer does not replace the warn with a user-facing error message:

```ts
function onBeforeUnload() {
  if (_pendingCount() === 0) return;
  console.warn(
    "[CardsClient] flushPendingDeletes on beforeunload — may be cancelled by browser",
  );
  void flushPendingDeletes();
}
```

**Why:** an unconditional warn desensitizes operators to the real signal — a non-zero pending count at unload time is worth triaging; a zero-pending unload is routine. The gate makes the warn a meaningful signal rather than noise. Reference: `frontend/src/app/cardgroups/[id]/cards/cards-client.tsx` `onBeforeUnload` after `_pendingCount` was exported from `frontend/src/lib/undo-delete.ts`.

### `fetchMoreError != null` halts the IO loop

Without an error halt gate, the IO keeps firing on the same failed cursor and loops invisibly with only `console.error` noise. Set state to a banner string in the `fetchMore` `.catch`, render a Retry button that clears the state and re-invokes the request, and short-circuit the IO `useEffect` while the error is set.

### `NetworkStatus.fetchMore`, not magic number

Always import the named `NetworkStatus` enum from `@apollo/client`. Magic numbers silently rot if Apollo renumbers (vanishingly rare, but the named import costs nothing).

### Capture `console.warn` for MockedProvider leaks, then assert in teardown

The assertion `nextPageCalls === 1` is partially tautological — `MockedProvider` matches per-entry, single-use, keyed on `(query, variables)`, so a leaked second `fetchMore` does not throw. It prints `"No more mocked responses for the query"` to `console.warn` and the `useQuery` hook resolves with `undefined` `data`; tests that depend on the second-page data thus silently pass on stale or missing data. The contract is **capture-then-explicit-assert**, not auto-fail-on-warn: `installApolloMockLeakSpy({ operationNames })` in `frontend/__tests__/utils/mock-apollo-paginated.ts` records every matching warning, and a `assertNoLeaks()` call in `afterEach` converts the captured set into a hard test failure. Restore the spy in the same `afterEach`. Without this, double-fetch regressions pass the call-count assertion silently.

#### Single-mock behavioral test for the in-flight guard

The leak-spy infrastructure also lets a test assert "the production guard prevents a double-fire" without re-implementing the guard in the test. Stage exactly **one** `nextPageMock` and trigger the IO callback twice synchronously within the same tick:

```ts
const nextPageMock = {
  request: { query: MyConnDocument, variables: { first: PAGE_SIZE, after: lastCursor } },
  result: () => { nextPageCalls += 1; return { data: { ... } }; },
};
renderClient([initialMock, nextPageMock]);
fireIntersect();
fireIntersect(); // synchronous second fire — the production fetchingRef must swallow it
await waitFor(() => expect(screen.getByText("page 2 item")).toBeInTheDocument());
expect(nextPageCalls).toBe(1);
```

If the production `useRef<boolean>` guard is missing or broken, the second `fireIntersect` causes a second `fetchMore` request, no second mock matches, and `MockedProvider` warns. `assertNoLeaks()` in `afterEach` turns that warning into a hard failure. The `nextPageCalls === 1` assertion alone is the partial-tautology case described above — the leak spy is what closes the loop. Reference: `frontend/src/app/cardgroups/cardgroups-client.test.tsx` (`does not fire fetchMore twice when sentinel intersects in the same animation frame`).

#### Spy stacking: install order is outer-first, teardown is LIFO

When a test file installs the leak spy AND a second `vi.spyOn(console, "warn")` (e.g. to assert that a non-Apollo `console.warn("[scope] ...")` call also fires), the second `spyOn` becomes the **outer** spy: every `console.warn(...)` call hits it first, and only forwards to the leak spy if the outer spy's `mockImplementation` does so. Two consequences:

1. **Do not call `outer.mockImplementation(() => {})` on the outer spy** — it swallows every warning before the leak spy records it, and `assertNoLeaks()` becomes a no-op while real leaks ship to production unnoticed. The `cards-new-client.test.tsx` regression that exposed this rule: a `mockImplementation(() => {})` was added to silence persist-failure warnings in a single test, and the leak spy stopped catching unmatched mocks for every test in the file.
2. **Restore in LIFO order**: outer spy first (so `console.warn` is back to the leak spy's mock), then the leak spy (so `console.warn` is back to the real implementation). Reversing leaves the leak spy's mock installed permanently.

If a single test needs to suppress a specific `console.warn` call, prefer asserting it explicitly via `expect(consoleWarnSpy).toHaveBeenCalledWith(...)` — the assertion documents intent and the call still flows through to the leak spy. The leak spy itself defaults to `silent: true` (`installApolloMockLeakSpy` swallows the formatted leak warning to keep CI output clean), so the outer spy does not need its own silencer.

### Stabilise `requestNextPage` via the cursor / search / hasNextPage ref triplet

The `useCallback` for `requestNextPage` reads three values that change every time a page lands or the user types into the search box: `endCursor`, `searchQuery`, and `hasNextPage`. If any of them appear in the callback's dep array, the callback gets a fresh identity on every advance — and the IntersectionObserver `useEffect` (which lists `requestNextPage` as a dep) tears down and re-attaches the observer on every page transition. The fix is to mirror all three values into refs and read them via `*.current` inside the callback, leaving only Apollo's stable `fetchMore` (and any cardgroup-id parameter) in the dep array:

```tsx
const endCursorRef = useRef(endCursor);
const searchQueryRef = useRef(searchQuery);
const hasNextPageRef = useRef(hasNextPage);
useEffect(() => { endCursorRef.current = endCursor; }, [endCursor]);
useEffect(() => { searchQueryRef.current = searchQuery; }, [searchQuery]);
useEffect(() => { hasNextPageRef.current = hasNextPage; }, [hasNextPage]);

const requestNextPage = useCallback(() => {
  if (fetchingRef.current || !hasNextPageRef.current) return;
  fetchingRef.current = true;
  fetchMore({
    variables: {
      ...DEFAULT_VARS,
      after: endCursorRef.current,
      search: searchQueryRef.current,
    },
    /* ... */
  })
    .then(() => setFetchMoreError(null))
    .catch((err) => { /* ...structured warn + banner... */ })
    .finally(() => { fetchingRef.current = false; });
}, [fetchMore]); // stable identity across page advances + search-text edits

useEffect(() => {
  if (!hasNextPage || fetchMoreError != null) return;
  const node = sentinelRef.current;
  if (!node) return;
  const observer = new IntersectionObserver((entries) => {
    if (!entries[0]?.isIntersecting || fetchingRef.current) return;
    requestNextPage();
  });
  observer.observe(node);
  return () => observer.disconnect();
}, [hasNextPage, fetchMoreError, requestNextPage]);
```

**Why apply all three refs together, not just one:** mirroring only `endCursor` while leaving `searchQuery` in the dep array still re-creates the callback on every keystroke; mirroring only `searchQuery` still re-creates it on every page advance. The triplet is the minimal stable set: `endCursor` advances per page, `searchQuery` changes per debounced input, `hasNextPage` flips when the last page is reached. The observer effect's structural deps (`hasNextPage`, `fetchMoreError`) are intentionally still real deps — those are the conditions that should re-subscribe the observer. Dropping `hasNextPage` from the effect's dep array would leave the observer attached past the last page; the rule keeps `hasNextPage` as a structural dep AND mirrors it into a ref so the callback's runtime check reads the current value without taking a per-advance dep.

**How to apply:** any paginated client component that subscribes to an IntersectionObserver and advances via `fetchMore` MUST apply the triplet. Today's call sites are `frontend/src/app/cardgroups/cardgroups-client.tsx`, `frontend/src/app/cardgroups/[id]/cards/cards-client.tsx`, and `frontend/src/app/admin/users/AdminUsersClient.tsx`. New paginated components should follow the same shape from the start — keep the `requestNextPage` dep array narrow to `[fetchMore]` (plus any caller-stable scalar like `cardgroupId`) and mirror every state value the body reads. This rule extends § "IntersectionObserver in-flight guard via `useRef<boolean>`" above: that rule covers the boolean re-entrancy guard; this rule covers the cursor/search/hasNextPage cohort that drives the callback's identity.

### Synchronous SSR cache seed in the render body, not in a `useEffect`

The SSR-seed-into-Apollo-cache pattern has historically run inside a `useEffect(() => apollo.writeQuery(...), [apollo, initialConnection])` with a `seededRef` boolean to survive Strict Mode's double-mount. Running the seed in a post-render effect leaves a window between first paint and the effect firing during which `useQuery` (cache-first) finds the cache empty and may issue a network round-trip — defeating the point of the SSR seed.

The fix is to write the seed synchronously in the component body, gated by the same `seededRef` boolean. The ref is set synchronously, so Strict Mode's double-invoke still produces exactly one write:

```tsx
// frontend/src/app/cardgroups/cardgroups-client.tsx
const seededRef = useRef(false);

// Seed runs synchronously during render — no useEffect needed.
// CARDGROUPS_DEFAULT_VARS keeps the cache key identical to the SSR seed and
// the client useQuery — any mismatch silently splits the cache.
if (!seededRef.current && initialConnection != null) {
  seededRef.current = true;
  apollo.writeQuery({
    query: MyCardgroupsConnectionDocument,
    variables: CARDGROUPS_DEFAULT_VARS,
    data: { myCardgroupsConnection: initialConnection },
  });
}

const { data, fetchMore, /* ... */ } = useQuery(MyCardgroupsConnectionDocument, {
  variables: queryVariables,
  fetchPolicy: "cache-first",
  /* ... */
});
```

**Why:** "writes during render" is generally an anti-pattern because most writes are observable side effects that schedule re-renders. Apollo's `cache.writeQuery` is the rare exception: the write does not synchronously trigger a re-render in the calling component (`useQuery`'s subscription notifies on the next microtask), and the operation is idempotent — writing the same data to the same cache key twice produces the same cache state. The synchronous `seededRef` guard makes the second invocation a no-op cheap enough to ignore. The post-effect alternative is the worse trade: the effect fires in a microtask after the first paint, so `useQuery`'s first cache-first read happens against an empty cache and may issue an unnecessary network request.

**How to apply:** any RSC-seeded paginated page that lifts initial data into Apollo cache MUST seed synchronously in the render body, gated by a `useRef(false)` boolean. Do not introduce a `useEffect` for this purpose. Confirm the `variables` object in the `writeQuery` is the **same object identity** the client's `useQuery` is keyed on (the shared `<TYPE>_DEFAULT_VARS` const documented in § "Variables shape MUST match between SSR seed and client cache reads"); a mismatched variables shape means `useQuery` reads a different cache entry than the seed wrote. Reference: `frontend/src/app/cardgroups/cardgroups-client.tsx` (`if (!seededRef.current && initialConnection != null)` synchronous seed). This rule pairs with § "`cache.modify` skips non-existent fields" above: both push cache writes to the seam where the cache-key invariant is enforced.

### Split debounce from immediate-reset effects on the same input

A single effect that combines the debounced state update with the immediate-reset cleanup violates the "one effect, one synchronization" guideline AND silently delays the reset by the debounce window. The user starts typing, the prior page's `fetchingRef = true` and `fetchMoreError` banner remain in place for 300ms, and the IO observer continues to interpret the prior cursor as live during that window. Split into two effects keyed on different triggers — the input for the debounce, the resulting query for the reset:

```tsx
// frontend/src/app/admin/users/AdminUsersClient.tsx
// Debounce: update searchQuery 300ms after the last keystroke.
useEffect(() => {
  const timer = setTimeout(() => {
    setSearchQuery(searchInput.trim() || null);
  }, 300);
  return () => clearTimeout(timer);
}, [searchInput]);

// When the active search query changes, drop any in-flight guard and stale error
// banner immediately so the new query starts from a clean slate.
// biome-ignore lint/correctness/useExhaustiveDependencies: searchQuery is an intentional trigger dependency; it is not referenced in the body because the effect resets derived IO state, not searchQuery itself.
useEffect(() => {
  fetchingRef.current = false;
  setFetchMoreError(null);
}, [searchQuery]);
```

**Why:** the two effects synchronize different things. The debounce effect synchronizes `searchQuery` to `searchInput` with a delay. The reset effect synchronizes `fetchingRef` / `fetchMoreError` to `searchQuery` with no delay. Combining them into one timer ties the reset's timing to the debounce's timing for no reason, which means a stale fetchMore banner stays visible during the entire debounce window even though the user has clearly moved on. Splitting them lets each synchronization run on its own trigger.

**How to apply:** every paginated client component that has both (a) a debounced search-input → search-query pipeline and (b) IO observer reset state (`fetchingRef`, `fetchMoreError`) MUST use two separate effects. Today's call sites are `frontend/src/app/cardgroups/cardgroups-client.tsx`, `frontend/src/app/cardgroups/[id]/cards/cards-client.tsx`, and `frontend/src/app/admin/users/AdminUsersClient.tsx` — all three follow the split-effect shape. The `biome-ignore lint/correctness/useExhaustiveDependencies` comment on the reset effect is load-bearing (see § "Load-bearing `biome-ignore` comments" below) — do not remove it during cleanup.

### Load-bearing `biome-ignore` comments

A `biome-ignore lint/correctness/useExhaustiveDependencies: <intent>` comment on a `useEffect` whose dep array intentionally lists a trigger NOT read in the body (e.g. a reset effect keyed on `searchQuery` whose body only resets derived IO state, never reads `searchQuery` itself) is load-bearing: it documents the trigger-not-read intent so a future code-mod tool, lint-rule upgrade, or IDE quick-fix does not silently re-engage the rule and either (a) widen the dep array to values the effect must NOT depend on, or (b) flag the suppression as a candidate for removal because "the dep is unused inside the body."

**Why:** the lint rule's heuristic is "every value read inside the effect body must appear in the dep array." It does not have a counterpart heuristic for "every value in the dep array must be read inside the body" — listing a trigger you never read is intentional but invisible to the rule. The `biome-ignore` comment is what tells a reader (and a code-mod tool) why the suppression is correct and what would break if the suppression were removed. A reviewer who deletes the comment "because the rule does not fire" silently strips the documentation that prevents the next contributor from removing the trigger from the dep array.

**How to apply:** when adding or refactoring an effect whose dep array carries a trigger-not-read value, write the `biome-ignore` comment with two pieces: (1) the rule name (`lint/correctness/useExhaustiveDependencies`), and (2) a one-sentence intent that names which dep is the trigger AND what the body resets ("`searchQuery` is an intentional trigger dependency; the body resets derived IO state, not `searchQuery` itself"). Treat the comment as load-bearing during code-cleanup passes — do not delete it on the assumption that "the rule does not currently fire." Reference: the reset effects in the three paginated client components named above. This rule pairs with § "Split debounce from immediate-reset effects" above: that rule introduces the trigger-not-read effect; this rule keeps the suppression's documentation alive.

### Provide two `MockedResponse` entries to test a Retry-after-error path

`MockedProvider` serves entries in order, single-use. A test that mounts with only one `{ request, error }` mock can assert that the error UI appears, but clicking Retry fires `refetch()` — which consumes a second entry. With only one entry, `MockedProvider` prints `"No more mocked responses for the query"` to `console.warn` and the `useQuery` hook resolves with `undefined` data; the refetch path is never exercised in a way that can assert the success state. Supply two matching entries:

```ts
const retryMocks: MockedResponse[] = [
  { request: { query: MyQuery }, error: new Error("network failure") },
  { request: { query: MyQuery }, result: { data: { ... } } },
];
```

The first entry drives the initial error state; the second entry is consumed by `refetch()`. A single-entry test for the same scenario passes tautologically — the leak spy records the unmatched warn and `assertNoLeaks()` in `afterEach` converts it to a hard failure. Reference: `frontend/src/components/cardgroups/cardgroup-picker-sheet.test.tsx` tests S12 and S13.
