# Direct unit tests for shared helpers + directionality assertion

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

A shared helper that is exercised only through integration tests leaves its
boundary conditions unpinned. The integration test proves the system works
end-to-end; it does not systematically cover `want=0`, `nil`, `len==want`,
`len==want+1`, and other edge cases. Add a direct unit test for any helper
that encodes non-trivial trimming or boundary logic. When the contract is a
closed-form law, write it as a property
(see [`docs/backend.md` § "Property-based tests (rapid)"](../../backend.md#property-based-tests-rapid))
whose generator reaches every edge in the checklist below.

## Example: `TrimAndDetect`

```go
func TestTrimAndDetect_Property_TrimsOnlyOverflow(t *testing.T) {
    t.Parallel()
    rapid.Check(t, func(t *rapid.T) {
        items := rapid.SliceOfN(rapid.Int(), 0, 30).Draw(t, "items") // nil one draw in ten
        want := rapid.IntRange(-3, 35).Draw(t, "want")                // covers want<=0, len<want, len==want, len==want+1
        got, more := TrimAndDetect(items, want)
        if want > 0 && len(items) > want {
            require.True(t, more)
            require.Equal(t, items[:want], got)
            return
        }
        require.False(t, more)
        require.Equal(t, items, got)
    })
}
```

The full test, including the `nil` draw, is in `backend/internal/usecase/page_property_test.go`.

## Directionality assertion for symmetric helpers

When two functions encode opposite directions of the same operation (one trims
the tail of a slice, its sibling trims the head), add a single case that feeds
both the same input and asserts they return different results. That one
assertion is load-bearing: it catches a future refactor that accidentally makes
the two functions identical, which neither function's own table can detect
because each table checks its function only against itself.

## Symmetric branch coverage for error-classifying sibling pairs

When two helpers share the same structure but differ in one classification branch
(e.g. `authorizeCardgroupOrBadInput` vs `authorizeCardgroupOrUnauthenticated`, both
in `backend/internal/usecase/ownership.go`), every branch in one must be mirrored
in the other. The siblings differ only in the `ErrNotFound` arm — one returns
`ucerr.NewValidationError(...)`, the other returns `ucerr.ErrUnauthenticated` — but
they share the same happy path, non-owner path, infrastructure-error path, and
context-pass-through path. A test file that covers only the new branch in one
sibling leaves the parallel branches in the other silently uncovered.

Apply the mirror rule when a new test file is introduced for either helper:
map each branch of the covered sibling to the corresponding branch of the uncovered
one and add a test for each gap. For `authorizeCardgroup*` the full branch map is:

| Branch | `OrBadInput` test | `OrUnauthenticated` test |
|---|---|---|
| `ErrNotFound` | `NotFound_ReturnsValidationError` | `NotFound_ReturnsUnauthenticated` |
| `context.Canceled` | `PropagatesCancelled` | `PropagatesCancelled` |
| `context.DeadlineExceeded` | `PropagatesDeadlineExceeded` | `PropagatesDeadlineExceeded` |
| Non-owner | `NonOwner_ReturnsUnauthenticated` | `NonOwner_ReturnsUnauthenticated` |
| Success | `Success_ReturnsNil` | `Success_ReturnsNil` |
| Infra error | `InfraError_WrappedAsInternal` | `InfraError_WrappedAsInternal` |

This is the test-file variant of the "pre-existing inconsistency surfaced by an
adjacent edit" rule in `.claude/rules/scope-discipline.md`: the new test file made
the gap visible, so the same PR closes it.

## Checklist for shared helper unit tests

1. **`want=0` (or zero-value sentinel)** — verify the bypass / no-op path.
2. **`nil` and `empty` inputs** — verify the helper does not panic and returns
   a consistent shape (nil-in → nil-out, or empty-in → empty-out as documented).
3. **`len == want`** — exact page, no trimming, no `hasMore`.
4. **`len == want+1`** — the sentinel row is trimmed, `hasMore = true`.
5. **Symmetric pair** — if the helper has an opposite-direction (or reverse)
   sibling, add one directionality assertion as described above.
6. **Error-classifying sibling pair** — if the helper has an `Or<X>` / `Or<Y>`
   sibling, map every branch to the corresponding branch in the other and confirm
   each is covered (see "Symmetric branch coverage" above).

## Why not rely on integration tests alone

Integration tests are slow, require a real database, and exercise the helper
only through the values a single test scenario happens to produce. The `want=0`
bypass or the `nil`-input no-op may never be triggered by the integration suite.
A regression there goes undetected until a production caller hits the edge case.

The direct unit test is fast and documents the full contract in
one place that a reader can scan without tracing through the system.
