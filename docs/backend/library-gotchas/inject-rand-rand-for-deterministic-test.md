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
//  1. The new partition is fully shuffled; the review partition is shuffled
//     within same-phase runs (shuffleWithinPhase) using rng.
//  2. New and review cards are interleaved at the caller-supplied ratio
//     (ratio.NewShare new per ratio.ReviewShare review), review-first.
//
// rng must be non-nil. Tests inject a seeded *rand.Rand for deterministic
// order; production constructs one per session. ratio is the per-user
// new-vs-review interleave ratio (domain.NewCardRatio).
func (p *OrderingPolicy) Apply(due []domain.DueCard, rng *rand.Rand, ratio domain.NewCardRatio) []*domain.Card {
    if rng == nil {
        panic("domain/service: OrderingPolicy.Apply requires non-nil rng")
    }
    // ... partition, shuffleWithinPhase, interleave(newCards, reviewCards, ratio.NewShare(), ratio.ReviewShare()) ...
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

**Why not `rand.Intn` or `SQL RANDOM()` for arrangement:** `SQL RANDOM()` runs
inside the database and cannot be seeded from Go tests, making *ordering*
assertions impossible. The repository still uses `random()` for *selection* —
deciding which rows enter each due-card window — because the selection set is
asserted by membership, not order. Pulling the arrangement shuffle into
application code with an injected source lets tests pin the exact post-shuffle
order. See [discovery-first due ordering](../ddd-patterns/discovery-first-due-ordering.md)
for the selection-vs-arrangement split.

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

## Production anti-pattern: per-request PRNG seed inside the operation

`rand.New(rand.NewSource(time.Now().UnixNano()))` constructed inside an operation
that runs per HTTP request produces a fresh PRNG sequence every time. If the
operation's output is observed across multiple consecutive calls — a queue refresh,
a list refetch, or any session window where the client displays successive server
responses — the user sees different orderings of the same logical input set. This
violates the user's expectation that the relative order of cards already visible
is stable: "I see card A above card B → I swipe → A is gone, B should be on top."
A fresh seed breaks that contract silently because there is no error or log line;
the ordering just changes.

Three patterns fix this, in increasing simplicity:

- **Move the shuffle outside the request boundary.** Compute the order once at
  session start, cache it server-side (or in the client), and replay it on
  subsequent requests. New cards that enter the session get appended rather than
  reshuffled into the whole queue.
- **Deterministic seed.** Derive the seed from a stable composite key —
  `(userID, cardgroupID, dueBucket, sessionID)` — so two consecutive requests
  with the same input produce the same output. Same due bucket → same order;
  a genuinely new bucket → a genuinely new order. No state needs to be stored;
  the seed is reconstructed from the inputs each time.
- **Drop the shuffle entirely.** If a specific presentation order does not matter,
  return cards in a stable sort order (e.g. `ORDER BY due ASC, id ASC`). Stable
  sort eliminates the per-request variation without any seed management.

**Worked example (issue #229).** `backend/internal/usecase/swipe.go` held a
`randSource` factory that called `rand.New(rand.NewSource(time.Now().UnixNano()))`
on every `HandleSwipe` invocation. The mutation response included a `nextCards`
field carrying a freshly shuffled queue snapshot. The frontend replaced its local
queue with that snapshot on each mutation response. The result: after swiping card
A, the optimistic UI removed A and showed B at the top; ~30–80 ms later the server
response arrived with a reshuffled queue and C appeared at the top instead — two
cards looked consumed from a single swipe. The fix dropped `nextCards` from the
mutation response entirely (the client now manages its own queue), which made the
seed strategy question moot. Had the field been retained, one of the three patterns
above would have been required to stabilize the order across the swipe + response
cycle.

**Anti-pattern grep.** Any production hit for the following pattern is a candidate
for review:

```bash
grep -rn 'rand\.NewSource(time\.Now' backend/ --include='*.go' | grep -v '_test.go'
```

A `_test.go` hit is the correct test-wiring pattern (fixed seed for determinism).
A non-test hit means a fresh random sequence is produced per call and should be
evaluated against the three patterns above.

**Related:**

- [`docs/backend/ddd-patterns/mutation-response-must-not-carry-client-managed-collection.md`](../ddd-patterns/mutation-response-must-not-carry-client-managed-collection.md) —
  the architectural fix that made the seed question moot for issue #229: once the
  mutation response no longer carries a queue snapshot, the server's shuffle order
  is never observed by the client across requests.
- This doc's existing section on test injection — the `seededRand` pattern (fixed
  seed in tests, factory in production) is the correct mirror: the production factory
  must produce a seed that is stable across the observation window, not freshly
  computed per request.
