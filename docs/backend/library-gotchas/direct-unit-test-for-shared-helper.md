# Direct unit tests for shared helpers + directionality assertion

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

A shared helper that is exercised only through integration tests leaves its
boundary conditions unpinned. The integration test proves the system works
end-to-end; it does not systematically cover `want=0`, `nil`, `len==want`,
`len==want+1`, and other edge cases. Add a direct unit test table for any
helper that encodes non-trivial trimming or boundary logic.

## Example: `TrimAndDetect` and `TrimAndDetectBackward`

```go
func TestTrimAndDetect(t *testing.T) {
    t.Parallel()
    tests := []struct {
        name     string
        in       []int
        want     int
        expected []int
        hasMore  bool
    }{
        // want=0: bypass all trimming, no hasMore signal
        {name: "want=0", in: []int{1, 2, 3}, want: 0, expected: []int{1, 2, 3}, hasMore: false},
        // fewer rows than requested: no trim needed
        {name: "len < want", in: []int{1, 2}, want: 5, expected: []int{1, 2}, hasMore: false},
        // exact page: no trim, no hasMore
        {name: "len == want", in: []int{1, 2, 3}, want: 3, expected: []int{1, 2, 3}, hasMore: false},
        // +1 row: trim sentinel, hasMore=true
        {name: "len == want+1", in: []int{1, 2, 3, 4}, want: 3, expected: []int{1, 2, 3}, hasMore: true},
        // more than +1: still hasMore=true, trim to want
        {name: "len > want+1", in: []int{1, 2, 3, 4, 5}, want: 3, expected: []int{1, 2, 3}, hasMore: true},
        // empty slice: no-op
        {name: "empty", in: []int{}, want: 3, expected: []int{}, hasMore: false},
        // nil slice: no-op, preserves nil
        {name: "nil", in: []int(nil), want: 3, expected: []int(nil), hasMore: false},
    }
    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            t.Parallel()
            got, hasMore := TrimAndDetect(tc.in, tc.want)
            require.Equal(t, tc.expected, got)
            require.Equal(t, tc.hasMore, hasMore)
        })
    }
}
```

## Directionality assertion for symmetric helpers

When two functions encode opposite directions (`TrimAndDetect` trims the tail;
`TrimAndDetectBackward` trims the head), add a single case that proves the
functions differ:

```go
func TestTrimAndDetect_DirectionDistinction(t *testing.T) {
    t.Parallel()
    in := []int{1, 2, 3, 4} // want=3, len==want+1 → hasMore on both

    forward, forwardMore := TrimAndDetect(in, 3)
    backward, backwardMore := TrimAndDetectBackward(in, 3)

    // forward trims tail → keeps leading elements
    require.Equal(t, []int{1, 2, 3}, forward)
    require.True(t, forwardMore)

    // backward trims head → keeps trailing elements
    require.Equal(t, []int{2, 3, 4}, backward)
    require.True(t, backwardMore)
}
```

This single assertion is load-bearing: it catches a future refactor that
accidentally makes the two functions identical.

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

## Field-by-field projection: distinct values per field catch transposition

A shared mapper that projects a domain struct onto a model struct field-by-field
(`toModelUserCardStateFromFSRS` in `backend/graph/resolver/mapper.go`, which
`toModelUserCardState` and `toSwipeResponseModel` both delegate to) has no
trimming or boundary logic — its failure mode is a *transposition*: two fields of
the same Go type (e.g. two `int`s, two `time.Time`s) swapped at the assignment
site. An integration test that selects only `{ stability state }` cannot catch a
swap among the fields it does not read.

Pin the projection directly with a unit test that gives every source field a
distinct non-zero value and asserts each target field equals its matching source,
labelling each assertion with the field name. Distinct values are load-bearing:
two fields holding the same value would let a transposition pass.
`TestToModelUserCardStateFromFSRS_ProjectsEveryField` is the worked example.

## Checklist for shared helper unit tests

1. **`want=0` (or zero-value sentinel)** — verify the bypass / no-op path.
2. **`nil` and `empty` inputs** — verify the helper does not panic and returns
   a consistent shape (nil-in → nil-out, or empty-in → empty-out as documented).
3. **`len == want`** — exact page, no trimming, no `hasMore`.
4. **`len == want+1`** — the sentinel row is trimmed, `hasMore = true`.
5. **Symmetric pair** — if the helper has a `Backward` (or `Reverse`) sibling,
   add one directionality assertion as described above.
6. **Error-classifying sibling pair** — if the helper has an `Or<X>` / `Or<Y>`
   sibling, map every branch to the corresponding branch in the other and confirm
   each is covered (see "Symmetric branch coverage" above).
7. **Field-by-field projection** — if the helper maps one struct onto another
   field-by-field, give every source field a distinct non-zero value and assert
   each target field individually to catch a transposition (see "Field-by-field
   projection" above).

## Why not rely on integration tests alone

Integration tests are slow, require a real database, and exercise the helper
only through the values a single test scenario happens to produce. The `want=0`
bypass or the `nil`-input no-op may never be triggered by the integration suite.
A regression there goes undetected until a production caller hits the edge case.

The direct unit test is fast, deterministic, and documents the full contract in
one place that a reader can scan without tracing through the system.
