# Interleave trailing-append paths need a non-divisible fixture per direction

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

A ratio-based interleave like `OrderingPolicy.Apply` (4 new per 1 review,
largest remainder) has two trailing-append paths that the main
`for i < len(newC) && j < len(reviewC)` loop never executes — one for each
bucket that runs empty first. Both paths must be exercised by fixtures whose
bucket sizes do NOT drain both buckets on the same slot.

```go
// production: interleave fills each slot from whichever bucket is furthest
// behind its share, then drains whichever bucket has cards left.
for i < len(newC) && j < len(reviewC) {
    if targetNewCount(len(out)+1, nRatio, den) > i {
        out = append(out, newC[i].Card); i++
    } else {
        out = append(out, reviewC[j].Card); j++
    }
}
// trailing-review path: only executes when newC empties first
for ; j < len(reviewC); j++ { out = append(out, reviewC[j].Card) }
// trailing-new path: only executes when reviewC empties first
for ; i < len(newC); i++ { out = append(out, newC[i].Card) }
```

A fixture that splits evenly — e.g. 16 new + 4 review at ratio 4:1 — drains
both buckets on the same slot. Neither trailing loop runs. A test built around
that fixture proves the ratio pattern but does NOT prove that either
trailing-append path emits cards in their post-shuffle order.

### Two fixtures per interleave

To cover both trailing paths, two distinct fixtures are required:

| Fixture | Cards | Path exercised |
|---|---|---|
| `1 new + 7 review` | new bucket empties on slot 1, review bucket has trailing surplus | trailing-review append |
| `10 new + 2 review` | review bucket empties on slot 8, then new bucket has trailing surplus | trailing-new append |

```go
// Trailing-review: 1N + 7R at 4:1 → slot 1 goes to new-0 and empties the new
// bucket; the main loop exits and trailing emits rev-0..rev-6.
in := []domain.DueCard{
    dueCard("rev-0", ..., t0), ..., dueCard("rev-6", ..., t6),
    dueCard("new-0", ..., t100),
}

// Trailing-new: 10N + 2R at 4:1 → slots 3 and 8 go to the review bucket and
// empty it; the main loop exits and trailing emits new-6..new-9.
in := []domain.DueCard{
    dueCard("rev-0", ..., t0), dueCard("rev-1", ..., t1),
    dueCard("new-0", ..., t100), ..., dueCard("new-9", ..., t109),
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
`TestInterleave_TrailingReviewAppend` (1N+7R fixture) and
`TestInterleave_TrailingNewAppend` (10N+2R fixture) cover the two
trailing paths. `backend/internal/usecase/swipe_performance_test.go` carries
the inline comment `Phase: FSRSPhaseReview exercises the review-bucket path`
at every `findDueRows` fixture that needs the review bucket populated.
