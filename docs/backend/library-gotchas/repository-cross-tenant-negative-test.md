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
