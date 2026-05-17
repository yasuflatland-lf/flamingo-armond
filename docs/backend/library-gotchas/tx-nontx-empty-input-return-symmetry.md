# Tx and non-Tx repository methods must return the same shape on empty input

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

When a repository exposes both `FindXxx` and `FindXxxTx` variants that accept a slice key, both must return the **same empty container** (not `nil`) when the input slice is empty. Asymmetric returns invite nil-check drift downstream: a caller that branches on `result == nil` vs `len(result) == 0` produces different behavior depending on which variant it called.

```go
// Correct — both variants return an empty map, never nil.
func (r *userCardFSRSRepo) FindByUserAndCardIDs(
    ctx context.Context, userID string, cardIDs []string,
) (map[string]*domain.UserCardFSRS, error) {
    if len(cardIDs) == 0 {
        return map[string]*domain.UserCardFSRS{}, nil
    }
    // ...
}

func (r *userCardFSRSRepo) FindByUserAndCardIDsTx(
    ctx context.Context, tx *gorm.DB, userID string, cardIDs []string,
) (map[string]*domain.UserCardFSRS, error) {
    if len(cardIDs) == 0 {
        return map[string]*domain.UserCardFSRS{}, nil
    }
    // ...
}
```

## Why this matters

- **Nil-check drift.** A `nil` map and an empty map both read as the zero value for unknown keys, so most callers tolerate either. But a single defensive `if result == nil` branch — added later by a different author — silently diverges between the Tx and non-Tx variants. The asymmetry is invisible at the call site and only surfaces as a behavioral difference in production.
- **Test coverage clarity.** The contract is testable via `require.NotNil(t, got); require.Empty(t, got)`. The `NotNil` assertion is load-bearing: it pins the empty-container shape the API promises. `require.Empty` alone tolerates `nil` and does not enforce the contract.
- **Future caller safety.** New consumers — DataLoader batch functions, JSON serializers — may behave differently on `nil` vs empty (`omitempty`, JSON-marshal `null` vs `{}`). Returning the empty container removes the ambiguity at the API surface.

## How to test the contract

Add a `_EmptySlice` test for each variant. The non-Tx test calls the method directly; the Tx test wraps the call in a `Transaction` callback:

```go
func TestUserCardFSRSRepository_FindByUserAndCardIDs_EmptySlice(t *testing.T) {
    t.Parallel()
    ctx := context.Background()
    ucsRepo := repository.NewUserCardFSRSRepository(testDB.GORM)

    got, err := ucsRepo.FindByUserAndCardIDs(ctx, "00000000-0000-0000-0000-000000000001", nil)
    require.NoError(t, err)
    require.NotNil(t, got)  // load-bearing: pins the empty-map contract
    require.Empty(t, got)
}

func TestUserCardFSRSRepository_FindByUserAndCardIDsTx_EmptySlice(t *testing.T) {
    t.Parallel()
    ctx := context.Background()
    ucsRepo := repository.NewUserCardFSRSRepository(testDB.GORM)

    var got map[string]*domain.UserCardFSRS
    err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        var inner error
        got, inner = ucsRepo.FindByUserAndCardIDsTx(ctx, tx, "00000000-0000-0000-0000-000000000001", nil)
        return inner
    })
    require.NoError(t, err)
    require.NotNil(t, got)  // load-bearing: pins the empty-map contract
    require.Empty(t, got)
}
```

`require.NotNil` is the load-bearing assertion. `require.Empty` alone tolerates `nil`.

## Generalizing the pattern

The same rule applies to any collection-returning repository method that has both Tx and non-Tx variants:

- Slice-returning methods: return `[]*domain.T{}`, not `nil`.
- Map-returning methods: return `map[K]*domain.T{}`, not `nil`.

When the Tx and non-Tx variants share a private helper (see [Tx and non-Tx repository methods share a private helper](repo-tx-and-nontx-share-private-helper.md)), the empty-input guard belongs in the helper — both public variants inherit it automatically and cannot diverge.

## Related rules

- [GORM `WHERE id IN ?` with empty slice returns all rows](../../../.claude/rules/go-library-gotchas.md) — the empty-keys guard at the repository boundary protects against a full-table scan; both rules share the principle "explicitly handle the empty-input edge case".
- [Tx and non-Tx repository methods share a private helper](repo-tx-and-nontx-share-private-helper.md) — when the shared helper owns the empty-input guard, both variants are guaranteed to return the same shape.

## Reference

- `backend/internal/repository/user_card_fsrs.go` — `FindByUserAndCardIDs` and `FindByUserAndCardIDsTx`, both returning `map[string]*domain.UserCardFSRS{}, nil` on empty input
- `backend/internal/repository/user_card_fsrs_test.go` — `TestUserCardFSRSRepository_FindByUserAndCardIDs_EmptySlice` and `TestUserCardFSRSRepository_FindByUserAndCardIDsTx_EmptySlice`, both asserting `require.NotNil(t, got)`
