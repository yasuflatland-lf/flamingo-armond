# Exact-boundary fixture for strict time-cutoff predicates, proven by temporary mutation

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

A repository predicate that compares against a cutoff with a *strict* operator
— `ucs.last_review < ?`, `created_at >= ?`, `due <= ?` — encodes an
inclusive/exclusive boundary decision (`<` vs `<=`, `>` vs `>=`) that ±1h
fixtures cannot pin. A row one hour before and a row one hour after the cutoff
both pass under either operator; only a row whose timestamp equals the cutoff
**exactly** distinguishes `<` from `<=`.

## The exact-boundary fixture

Add a third fixture whose timestamp is the cutoff value verbatim and assert the
boundary side it lands on. For a strict `<` cutoff the equal-to-boundary row is
excluded:

```go
boundary := domain.NewUserCardFSRSForNewCard(ownerID, reviewedAtBoundary.ID, now)
boundary.State.Phase = domain.FSRSPhaseLearning
boundary.State.Due = now.Add(-time.Hour)
boundary.State.LastReview = startOfToday // exactly at boundary → excluded under strict <
require.NoError(t, ucsRepo.UpsertTx(ctx, tx, boundary))
// ...
require.Equal(t, []string{reviewedYesterday.ID}, repoCardIDs(got),
    "only cards reviewed strictly before the boundary enter the review window")
```

Worked example: `TestCardRepository_FindDueCards_ExcludesCardsReviewedToday`
(`backend/internal/repository/card_test.go`) carries the `reviewed-yesterday`
(before), `reviewed-today` (after), and `reviewed-at-boundary` (equal) trio.
The predicate it guards is `ucs.last_review < ?` in `findDueCardsOn`
(`backend/internal/repository/card.go`).

## Prove a new regression pin actually bites

A test that passes on first run proves nothing on its own — the production code
may already be correct, or the assertion may be vacuous. Before trusting a new
pin, **temporarily mutate the production line it guards**, confirm the test
FAILS, then restore and confirm the diff is clean:

- For the strict-`<` boundary above: change `ucs.last_review < ?` to
  `ucs.last_review <= ?`. The exact-boundary row now passes the predicate and
  enters the review window, so the `require.Equal` against the single expected
  ID fails. Restore the `<` and the test goes green again.
- For a JST start-of-day cutoff: change `domain.StartOfLearnDay(now)` to a raw `now`
  (`backend/internal/usecase/learn.go`). A card reviewed earlier today now sits
  before the looser cutoff and reappears in the queue, breaking the
  exclusion assertion.

The mutation must produce a *failing* test, not merely a different one. A pin
that survives the mutation is not testing the boundary — widen the fixture until
the mutation breaks it. Delete nothing permanently; the mutation is a throwaway
check, and the committed tree must show no production change.

## Complementary two-window boundary

When two predicates partition a timeline around a single cutoff — the learn
window keeps `last_review < boundary` and the practice window keeps
`last_review >= boundary` — the two comparators are the *complement* of each
other across that one instant. Pinning them in separate tests with separate
boundary values leaves a gap: nothing proves that the two windows agree on where
the cutoff sits, so a card at the exact boundary could end up in both windows or
in neither without any single test noticing.

Pin both predicates in **one** test against **one** boundary instant, and assert
two properties jointly:

- **Mutual exclusion + joint exhaustiveness.** A card whose `last_review` equals
  the boundary belongs to exactly one window (here practice, because `>=` is
  inclusive and `<` is exclusive). A never-reviewed card (NULL `last_review`)
  belongs to neither window's `last_review` predicate — it reaches the learn
  queue only through the separate new-card window (see
  [`sql-null-comparison-excludes-unreviewed-rows.md`](sql-null-comparison-excludes-unreviewed-rows.md)).

`TestCardRepository_FindPracticeCards_BoundaryComplementarity`
(`backend/internal/repository/card_test.go`) is the worked example. It inserts
four cards reviewed before / at / after one boundary plus one never-reviewed
card, then asserts the learn window holds `{reviewed-yesterday, never-reviewed}`
and the practice pool holds exactly `{reviewed-at-boundary, reviewed-today}`.
Two mutation kills prove the comparators independently:

- Flip the practice `ucs.last_review >= ?` to `>`: `reviewed-at-boundary` drops
  out of the practice pool, failing the practice `ElementsMatch`.
- Flip the learn `ucs.last_review < ?` to `<=`: `reviewed-at-boundary` enters
  the learn window, failing the learn `ElementsMatch`.

Both predicates live in `findPracticeCardsOn` / `findDueCardsOn`
(`backend/internal/repository/card.go`).
