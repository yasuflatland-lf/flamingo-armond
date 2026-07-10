# Interleave trailing-append paths need a non-divisible fixture per direction

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

A ratio-based interleave like `OrderingPolicy.Apply` (1 new per 4 review,
review-first) has two trailing-append paths that the outer `for i < len(newC)
&& j < len(reviewC)` loop never executes — one for each bucket that runs
empty first. Both paths must be exercised by fixtures whose bucket sizes do
NOT split evenly across the ratio cycle.

```go
// production: interleave emits rRatio reviews then nRatio news per cycle,
// then drains whichever bucket has cards left.
for i < len(newC) && j < len(reviewC) {
    for k := 0; k < rRatio && j < len(reviewC); k++ {
        out = append(out, reviewC[j].Card); j++
    }
    for k := 0; k < nRatio && i < len(newC); k++ {
        out = append(out, newC[i].Card); i++
    }
}
// trailing-review path: only executes when newC empties first
for ; j < len(reviewC); j++ { out = append(out, reviewC[j].Card) }
// trailing-new path: only executes when reviewC empties first
for ; i < len(newC); i++ { out = append(out, newC[i].Card) }
```

A fixture that splits evenly — e.g. 5 new + 20 review at ratio 1:4 — drains
both buckets simultaneously on the last cycle. Neither trailing loop runs.
A test built around that fixture proves the ratio pattern but does NOT prove
that either trailing-append path emits cards in their post-shuffle order.

### Two fixtures per interleave

To cover both trailing paths, two distinct fixtures are required:

| Fixture | Cards | Path exercised |
|---|---|---|
| `1 new + 7 review` | new bucket empties mid-loop, review bucket has trailing surplus | trailing-review append |
| `5 new + 3 review` | review bucket empties inside the k-loop guard, then new bucket has trailing surplus | trailing-new append |

```go
// Trailing-review: 1N + 7R → first cycle emits rev-0..rev-3 + new-0,
// outer loop exits when i==1==len(newC), trailing emits rev-4..rev-6.
in := []domain.DueCard{
    dueCard("rev-0", ..., t0), ..., dueCard("rev-6", ..., t6),
    dueCard("new-0", ..., t100),
}

// Trailing-new: 5N + 3R → k-loop emits rev-0..rev-2 then exits via the
// "j < len(reviewC)" guard, new-0 emitted, outer loop exits when
// j==3==len(reviewC), trailing emits new-1..new-4.
in := []domain.DueCard{
    dueCard("rev-0", ..., t0), ..., dueCard("rev-2", ..., t2),
    dueCard("new-0", ..., t100), ..., dueCard("new-4", ..., t104),
}
```

### Phase zero-value collapses the partition

A separate but related pitfall: `domain.DueCard{Card: c}` leaves
`Phase` at its zero value (`FSRSPhaseNew`). A test built only from
zero-value fixtures exercises ONLY the new-bucket branch of `partition` —
the review-bucket branch is never entered, and no interleave path runs at
all because the review bucket is empty. Any fixture intended to exercise
the review-bucket path or the interleave loop MUST set
`Phase: FSRSPhaseReview` (or `Learning` / `Relearning`) explicitly:

```go
// Exercises review-bucket: required for interleave coverage.
domain.DueCard{Card: &domain.Card{ID: "next-1"}, Phase: domain.FSRSPhaseReview}
```

**Reference:** `backend/internal/domain/service/due_card_ordering_test.go` —
`TestOrderingPolicy_Apply_TrailingReviewAppend` (1N+7R fixture) and
`TestOrderingPolicy_Apply_TrailingNewAppend` (5N+3R fixture) cover the two
trailing paths. `backend/internal/usecase/swipe_performance_test.go` carries
the inline comment `Phase: FSRSPhaseReview exercises the review-bucket path`
at every `findDueRows` fixture that needs the review bucket populated.
