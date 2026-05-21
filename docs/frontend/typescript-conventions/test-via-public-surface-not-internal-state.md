# Test hooks via their public surface, not their internal state

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

A hook test that asserts "calling `setState` produces the expected `state` value" tests React, not the hook. The hook under test is its **public contract**: which inputs drive it, which outputs it exposes, and which externally-observable transitions those inputs trigger. Internal state setters are an implementation detail; exposing them through the public API is only justified when they represent a user-facing action, not a testing convenience.

## Wrong: pin the setter itself

```tsx
// useCardsConnection exposes setFetchMoreError as part of its return value.
// This test drives the hook by calling the setter directly.
it("fetchMoreError updates when set", () => {
  const { result } = renderHook(() => useCardsConnection(mockArgs));

  act(() => {
    result.current.setFetchMoreError("manual banner");
  });

  expect(result.current.fetchMoreError).toBe("manual banner");
});
```

This test passes as long as React's `useState` works. It does not verify the hook's invariant — "`fetchMoreError != null` halts the IO loop" — and it would pass even if the IO loop were broken or missing entirely.

## Right: drive the hook through its real input path

```tsx
// Provide a MockedProvider that rejects fetchMore.
// Assert that after the rejection, fetchMoreError is set AND the IO loop halts.
it("fetchMoreError halts the IO loop after a failed fetchMore", async () => {
  const mocks = [
    buildInitialQueryMock(),          // first load succeeds
    buildFetchMoreRejectionMock(),     // fetchMore rejects
  ];

  const { result } = renderHook(
    () => useCardsConnection({ cardgroupId: "cg-1" }),
    { wrapper: buildWrapper(mocks) },
  );

  // Trigger the first page load.
  await waitFor(() => expect(result.current.cards.length).toBeGreaterThan(0));

  // Trigger fetchMore — the mock rejects.
  act(() => result.current.requestNextPage());
  await waitFor(() => expect(result.current.fetchMoreError).not.toBeNull());

  // The IO loop must not fire again while the error is set.
  const callCountBefore = mockFetchMore.mock.calls.length;
  act(() => result.current.requestNextPage());
  expect(mockFetchMore).toHaveBeenCalledTimes(callCountBefore); // loop halted
});
```

See [`docs/pagination/two-mocked-responses-for-retry-test.md`](../../pagination/two-mocked-responses-for-retry-test.md) for the two-mock pattern that covers the Retry-after-error path in the same suite.

## Why

Exposing `setFetchMoreError` through the hook's return value creates an API surface that callers can misuse: any consumer can clear or override the error state without going through the intended `retryFetchMore` action. The setter test reinforces that misuse by building a test harness around the low-level primitive instead of the invariant it is meant to enforce.

The asymmetry to watch for: if removing a test causes no invariant to go uncovered, the test was exercising an implementation detail, not the hook's contract. Apply the "would this test catch a bug in the hook's stated purpose?" filter before writing. A test that fails when the IO loop keeps firing despite a non-null error is load-bearing; a test that fails when React's `useState` is broken is not.

**How to apply:** write the test against the hook's stated purpose — the documented invariant in the JSDoc or the hook's README section. If the invariant cannot be triggered without exposing an internal setter, that is a signal to add a purpose-built action to the public API (e.g. `retryFetchMore` instead of `setFetchMoreError`) so the test can drive the hook naturally. See [`docs/pagination/two-mocked-responses-for-retry-test.md`](../../pagination/two-mocked-responses-for-retry-test.md) for the concrete Apollo-mock pattern.
