# Provide two `MockedResponse` entries to test a Retry-after-error path

> Part of the [pagination](../../.claude/rules/pagination.md) rules. Cross-referenced by `docs/backend.md` and `frontend/CLAUDE.md`.

`MockedProvider` serves entries in order, single-use. A test that mounts with only one `{ request, error }` mock can assert that the error UI appears, but clicking Retry fires `refetch()` — which consumes a second entry. With only one entry, `MockedProvider` prints `"No more mocked responses for the query"` to `console.warn` and the `useQuery` hook resolves with `undefined` data; the refetch path is never exercised in a way that can assert the success state. Supply two matching entries:

```ts
const retryMocks: MockedResponse[] = [
  { request: { query: MyQuery }, error: new Error("network failure") },
  { request: { query: MyQuery }, result: { data: { ... } } },
];
```

The first entry drives the initial error state; the second entry is consumed by `refetch()`. A single-entry test for the same scenario passes tautologically — the leak spy records the unmatched warn and `assertNoLeaks()` in `afterEach` converts it to a hard failure. Reference: `frontend/src/components/cardgroups/cardgroup-picker-sheet.test.tsx` tests S12 and S13.
