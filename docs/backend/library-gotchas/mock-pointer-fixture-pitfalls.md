# Mock-pointer fixture pitfalls: directionality tautology and parallel sub-test races

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

Two test-fixture hazards arise together whenever a mock returns a `*domain.X`
by value (the same pointer the test's outer scope holds) and the production
code mutates that struct in place. The tautology trap makes an assertion
silently dead; the parallel race turns it into undefined behaviour.

## The shared-pointer directionality tautology

### Why the assertion is dead code

When a test mock's `findResult` field holds the same `*domain.Card` pointer
that the outer scope uses as its fixture, and the production code mutates
that struct in place (e.g. via `card.UpdateFront(newValue)`), an assertion
of the form

```go
if *cardRepo.capturedPatch.Front != existingCard.Front.String() { /* fail */ }
```

is tautologically true. The execution sequence is:

1. The mock returns the outer-scope pointer.
2. The production code calls `existingCard.UpdateFront(newValue)`, writing
   `newValue` into `existingCard.Front` through that pointer.
3. The repository captures `existingCard.Front.String()` as `capturedPatch.Front`.
4. The assertion compares `*capturedPatch.Front` against `existingCard.Front.String()`.

Both sides were written by the same mutation in step 2. The assertion passes
whether or not the route-through actually ran. A regression that bypassed
`UpdateFront` and derived the patch value by some other path would leave
`existingCard.Front` at its initial value — but then `capturedPatch.Front`
would also reflect that unchanged value, so the comparison still holds. The
test cannot distinguish the regression it claims to catch.

### The fix — mutation guard against the initial known value

Snapshot the initial value before the call (or hard-code it as a literal),
then assert that the aggregate field changed *to the expected new value*. A
regression that skipped the aggregate-mutation step leaves the field at its
initial value, and the guard trips.

```go
existingFront := &domain.Card{
    ID:          "card1",
    CardgroupID: "cg1",
    Front:       domain.CardText("old front"),
    Back:        domain.CardText("old back"),
}
cardRepo := &mockCardRepository{findResult: existingFront, updateResult: existingFront}
uc := NewCardUsecase(nil, cardRepo, nil, nil)

newFront := " new front "
_, _ = uc.Update(ctx, "card1", UpdateCardInput{Front: &newFront})

// Tautology (do not use) — both sides equal "new front" because the mock
// returned existingFront and the usecase mutated it in place.
//   if *cardRepo.capturedPatch.Front != existingFront.Front.String() { ... }  // DEAD CODE

// Mutation guard (load-bearing) — a bypass-the-route-through regression
// leaves existingFront.Front at "old front" and fails this assertion.
if existingFront.Front != domain.CardText("new front") {
    t.Fatalf("aggregate not mutated: Front = %q, want %q",
        existingFront.Front, "new front")
}
```

The initial value `"old front"` is the anchor. If the production code ever
stops calling `card.UpdateFront` and drives the patch a different way, the
aggregate field stays at `"old front"` and the assertion fails — which is
exactly the regression the test exists to catch.

The shape generalises: any test that uses a mutable pointer as both mock
state and assertion subject must compare against the *initial known value*
(or a snapshot taken before the call under test), not against another value
produced by the same mutation.

## Parallel sub-test fixture isolation

### Why the race is silent

`t.Parallel()` schedules sibling sub-tests on goroutines that the runtime
may interleave. When two sub-tests share a `*domain.Card` pointer via an
outer-scope variable and both pass it as `mockCardRepository{findResult: ...}`,
any sub-test whose call path mutates that pointer races with any concurrent
reader.

The danger is silent for two reasons:

1. One sibling may return early on an auth check before reaching the mutation,
   keeping the race window narrow. The race detector may not fire under low
   contention.
2. Tests pass consistently on a local laptop but exhibit undefined behaviour
   under a different scheduler or heavier load.

The acceptance criterion `go test -race ./...` is supposed to catch this, but
race detection is probabilistic per run.

### The fix — each parallel sub-test owns its own fixture pointer

Declare a fresh `*domain.Card` inside the sub-test that mutates it. The
outer-scope fixture may remain shared if and only if all readers are read-only.

```go
func TestCardUsecase_Update_NonOwnerAndPatch(t *testing.T) {
    t.Parallel()

    // outer fixture — used read-only by the "non owner" sub-test below.
    existing := &domain.Card{
        ID:          "card1",
        CardgroupID: "cg1",
        Front:       domain.CardText("front"),
        Back:        domain.CardText("back"),
    }

    t.Run("non owner", func(t *testing.T) {
        t.Parallel()
        // Reads existing only; no mutation through this pointer.
        uc := NewCardUsecase(nil,
            &mockCardRepository{findResult: existing},
            nil, nil)
        _, err := uc.Update(ctx, "card1", UpdateCardInput{})
        assertUnauthenticated(t, err)
    })

    t.Run("valid partial update", func(t *testing.T) {
        t.Parallel()
        // Isolated fixture: sharing `existing` with the sibling above and
        // mutating through it would be a data race under -race.
        existingFront := &domain.Card{
            ID:          "card1",
            CardgroupID: "cg1",
            Front:       domain.CardText("old front"),
            Back:        domain.CardText("old back"),
        }
        cardRepo := &mockCardRepository{
            findResult:   existingFront,
            updateResult: existingFront,
        }
        uc := NewCardUsecase(nil, cardRepo, nil, nil)

        newFront := " new front "
        _, _ = uc.Update(ctx, "card1", UpdateCardInput{Front: &newFront})

        if existingFront.Front != domain.CardText("new front") {
            t.Fatalf("aggregate not mutated: Front = %q", existingFront.Front)
        }
    })
}
```

The same rule applies to `t.Run` siblings nested under any parallel parent,
not only top-level test functions.

## How to spot both pitfalls in review

- A sub-test uses `findResult: existing` where `existing` is declared in an
  outer scope AND the production call path mutates the returned aggregate in
  place — this is the trigger for both pitfalls simultaneously.
- An assertion of the form `*capturedPatch.X == existing.X.String()` after
  that call is the dead-code shape (directionality tautology).
- Two or more `t.Parallel()` sub-tests sharing that same outer-scope pointer
  is the race shape.

The fix is usually both corrections together:

1. Move the fixture inside the mutating sub-test (isolation).
2. Rewrite the assertion as a mutation guard against the literal initial value
   (directionality).

## Reference

- `backend/internal/usecase/card_test.go` — `TestCardUsecase_Update_NonOwnerAndPatch`.
  Both `"valid partial update"` and `"valid partial update back"` sub-tests use
  isolated fixtures; each mutation guard reads
  `existingFront.Front != domain.CardText(newFront)`.
- Issue #213 — `Card.UpdateFront` / `Card.UpdateBack` route-through that
  introduced the in-place mutation and surfaced both pitfalls.
