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

## Checklist for shared helper unit tests

1. **`want=0` (or zero-value sentinel)** — verify the bypass / no-op path.
2. **`nil` and `empty` inputs** — verify the helper does not panic and returns
   a consistent shape (nil-in → nil-out, or empty-in → empty-out as documented).
3. **`len == want`** — exact page, no trimming, no `hasMore`.
4. **`len == want+1`** — the sentinel row is trimmed, `hasMore = true`.
5. **Symmetric pair** — if the helper has a `Backward` (or `Reverse`) sibling,
   add one directionality assertion as described above.

## Why not rely on integration tests alone

Integration tests are slow, require a real database, and exercise the helper
only through the values a single test scenario happens to produce. The `want=0`
bypass or the `nil`-input no-op may never be triggered by the integration suite.
A regression there goes undetected until a production caller hits the edge case.

The direct unit test is fast, deterministic, and documents the full contract in
one place that a reader can scan without tracing through the system.
