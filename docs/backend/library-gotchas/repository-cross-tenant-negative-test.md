# Repository lookup methods scoped by tenant ID require a cross-tenant negative test

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

Any `FindBy*` method that includes a `cardgroup_id = ?` (or other tenant-scoping) predicate must be regression-guarded by inserting the same discriminating value into **two** separate tenant rows and asserting the result is the tenant-scoped row, not the other one. Without this guard, dropping or accidentally omitting the predicate in a refactor silently leaks another tenant's row through duplicate-detection or query logic:

```go
cardA := newCard(cgA.ID, "apple", "back-A")
cardB := newCard(cgB.ID, "apple", "back-B")
require.NoError(t, repo.Create(ctx, cardA))
require.NoError(t, repo.Create(ctx, cardB))

got, err := repo.FindByCardgroupAndFront(ctx, cgB.ID, "apple")
require.NoError(t, err)
require.Equal(t, cardB.ID, got.ID, "must return cardgroup B's card, not cardgroup A's")
```

Apply this pattern to any repository method whose correctness depends on a tenant-scoping predicate.

## Per-user JOIN variant: the discriminator must be load-bearing

The same guard applies to a `LEFT JOIN` scoped by user, not just a `WHERE`
predicate scoped by cardgroup. When a query joins a per-user table with
`ON ucs.user_id = ? AND ...`, insert **another user's** row for the same entity
and assert it does not perturb the calling user's result. The subtlety is
choosing a discriminating value: the inserted row must carry a value that would
*change the outcome* if the `user_id` predicate leaked, otherwise the test is
green for the wrong reason.

Worked example: `TestCardRepository_FindDueCards_IgnoresOtherUsersFSRSRows`
(`backend/internal/repository/card_test.go`) inserts a second user's
**future-due** FSRS row for a shared card. The join is `LEFT JOIN user_card_fsrs
ucs ON ucs.user_id = ? AND ucs.card_id = cards.id` in `findDueCardsOn`
(`backend/internal/repository/card.go`). The calling user has no FSRS row, so
their JOIN slot is NULL and the card should surface through the new-card window.
A future-due value is what makes the fixture discriminating: if the `user_id`
predicate leaked, the other user's row would attach to the card, its future
`due` would push it out of both the new window (now non-NULL) and the review
window (not yet due), so the card vanishes — a result a *present-due* or
*overdue* discriminator would not reliably produce. Pick the discriminator that
makes a leaked predicate visible, not just present.
