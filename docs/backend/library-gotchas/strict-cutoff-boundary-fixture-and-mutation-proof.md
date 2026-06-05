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
boundary.State.State = domain.FSRSStateLearning
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
- For a JST start-of-day cutoff: change `startOfDayJST(now)` to a raw `now`
  (`backend/internal/usecase/learn.go`). A card reviewed earlier today now sits
  before the looser cutoff and reappears in the queue, breaking the
  exclusion assertion.

The mutation must produce a *failing* test, not merely a different one. A pin
that survives the mutation is not testing the boundary — widen the fixture until
the mutation breaks it. Delete nothing permanently; the mutation is a throwaway
check, and the committed tree must show no production change.
