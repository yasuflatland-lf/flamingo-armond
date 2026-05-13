# Inject `*rand.Rand` into pure functions to keep tests deterministic

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

A pure function that calls `rand.Shuffle` on a global source cannot be tested
deterministically — the shuffle order varies per run. The fix is to accept
`*rand.Rand` as a parameter. Production code passes a time-seeded source; tests
pass a fixed seed and can assert an exact output order.

```go
// backend/internal/domain/service/due_card_ordering.go

// Apply returns cards ordered by due ASC, with same-due ties shuffled by r.
// Passing r == nil skips the shuffle (stable sort only).
// Apply is pure: it never mutates the input slice.
func (p *OrderingPolicy) Apply(cards []*domain.Card, r *rand.Rand) []*domain.Card {
    out := append([]*domain.Card(nil), cards...) // copy, never mutate input
    sort.SliceStable(out, func(i, j int) bool {
        return out[i].FSRS.Due.Before(out[j].FSRS.Due)
    })
    if r == nil {
        return out
    }
    // shuffle runs of cards sharing the same due timestamp
    for start := 0; start < len(out); {
        end := start + 1
        for end < len(out) && out[end].FSRS.Due.Equal(out[start].FSRS.Due) {
            end++
        }
        if end-start > 1 {
            run := out[start:end]
            r.Shuffle(len(run), func(i, j int) { run[i], run[j] = run[j], run[i] })
        }
        start = end
    }
    return out
}
```

The caller owns the `*rand.Rand` lifecycle. `LearnUsecase` stores a factory
function so each call gets a fresh source:

```go
// production wiring (time-seeded)
randSource: func() *rand.Rand {
    return rand.New(rand.NewSource(time.Now().UnixNano()))
}

// test wiring (fixed seed)
randSource: func() *rand.Rand {
    return rand.New(rand.NewSource(42))
}
```

**Why not `rand.Intn` or `SQL RANDOM()`:** `SQL RANDOM()` runs inside the database
and cannot be seeded from Go tests, making ordering assertions impossible.
Pulling the shuffle into application code with an injected source keeps the
database responsible only for fetching rows in stable due-date order
(`ORDER BY due ASC, id ASC`); the application layer applies the within-tie
shuffle on top.

**Passing `nil` skips the shuffle:** when the caller does not care about
tie-breaking order (integration tests asserting only which cards appear, not
their sequence within a tie), passing `r = nil` returns a stable sort and
removes the non-determinism without requiring a seeded source.

**Reference:** `backend/internal/domain/service/due_card_ordering.go` — `OrderingPolicy.Apply`;
`backend/internal/usecase/learn.go` — `randSource func() *rand.Rand` field and
`NewLearnUsecase` factory injection.
