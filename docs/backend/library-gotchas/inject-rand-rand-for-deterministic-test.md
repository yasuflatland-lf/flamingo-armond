# Inject `*rand.Rand` into pure functions to keep tests deterministic

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

A pure function that calls `rand.Shuffle` on a global source cannot be tested
deterministically — the shuffle order varies per run. The fix is to accept
`*rand.Rand` as a parameter. Production code passes a time-seeded source; tests
pass a fixed seed and can assert an exact output order.

```go
// backend/internal/domain/service/due_card_ordering.go

// Apply orders due cards with a two-step policy:
//
//  1. Within each partition (new / review), cards sharing the same Due
//     timestamp are shuffled using rng.
//  2. New and review cards are interleaved at NewCardRatio:ReviewCardRatio.
//
// rng must be non-nil. Tests inject a seeded *rand.Rand for deterministic
// order; production constructs one per session.
func (p *OrderingPolicy) Apply(due []domain.DueCard, rng *rand.Rand) []*domain.Card {
    if rng == nil {
        panic("domain/service: OrderingPolicy.Apply requires non-nil rng")
    }
    // ... partition, shuffleSameDue, interleave ...
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
(`ORDER BY COALESCE(ucs.due, cards.created_at) ASC, cards.id ASC`); the
application layer applies the within-tie shuffle on top.

**Non-nil rng is enforced at the entry point.** Earlier iterations of the
policy used `rng == nil` to mean "stable sort only". The current contract
panics on `nil` because every production wiring (and every test fixture
written since the interleave step landed) supplies a seeded source — silently
returning a stable order on `nil` would mask a wiring bug. Tests that want
deterministic output pass `rand.NewSource(42)`; tests that only assert set
equality still pass a seeded source.

**Reference:** `backend/internal/domain/service/due_card_ordering.go` — `OrderingPolicy.Apply`
panics on nil rng; `backend/internal/usecase/learn.go` — `randSource func() *rand.Rand`
field, factory pattern documented under
[constructor-relationship-invariant-panic](constructor-relationship-invariant-panic.md).
