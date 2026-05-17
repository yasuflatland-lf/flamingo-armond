# Capture `console.warn` for MockedProvider leaks, then assert in teardown

> Part of the [pagination](../../.claude/rules/pagination.md) rules. Cross-referenced by `docs/backend.md` and `frontend/CLAUDE.md`.

The assertion `nextPageCalls === 1` is partially tautological — `MockedProvider` matches per-entry, single-use, keyed on `(query, variables)`, so a leaked second `fetchMore` does not throw. It prints `"No more mocked responses for the query"` to `console.warn` and the `useQuery` hook resolves with `undefined` `data`; tests that depend on the second-page data thus silently pass on stale or missing data. The contract is **capture-then-explicit-assert**, not auto-fail-on-warn: `installApolloMockLeakSpy({ operationNames })` in `frontend/__tests__/utils/mock-apollo-paginated.ts` records every matching warning, and a `assertNoLeaks()` call in `afterEach` converts the captured set into a hard test failure. Restore the spy in the same `afterEach`. Without this, double-fetch regressions pass the call-count assertion silently.

## Single-mock behavioral test for the in-flight guard

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

## Spy stacking: install order is outer-first, teardown is LIFO

When a test file installs the leak spy AND a second `vi.spyOn(console, "warn")` (e.g. to assert that a non-Apollo `console.warn("[scope] ...")` call also fires), the second `spyOn` becomes the **outer** spy: every `console.warn(...)` call hits it first, and only forwards to the leak spy if the outer spy's `mockImplementation` does so. Two consequences:

1. **Do not call `outer.mockImplementation(() => {})` on the outer spy** — it swallows every warning before the leak spy records it, and `assertNoLeaks()` becomes a no-op while real leaks ship to production unnoticed. The `cards-new-client.test.tsx` regression that exposed this rule: a `mockImplementation(() => {})` was added to silence persist-failure warnings in a single test, and the leak spy stopped catching unmatched mocks for every test in the file.
2. **Restore in LIFO order**: outer spy first (so `console.warn` is back to the leak spy's mock), then the leak spy (so `console.warn` is back to the real implementation). Reversing leaves the leak spy's mock installed permanently.

If a single test needs to suppress a specific `console.warn` call, prefer asserting it explicitly via `expect(consoleWarnSpy).toHaveBeenCalledWith(...)` — the assertion documents intent and the call still flows through to the leak spy. The leak spy itself defaults to `silent: true` (`installApolloMockLeakSpy` swallows the formatted leak warning to keep CI output clean), so the outer spy does not need its own silencer.
