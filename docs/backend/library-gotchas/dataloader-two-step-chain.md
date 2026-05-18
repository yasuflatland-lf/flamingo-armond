# DataLoader two-step chain: hydrate aggregate A, then key aggregate B from its field

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

When a GraphQL field traverses two aggregate boundaries — e.g. `User.lastViewedCardgroup` reads a `UserPreference` row and then resolves the `Cardgroup` named by it — the natural single-loader approach fans out into N round-trips: N users in the response each trigger one preference lookup plus one cardgroup lookup. The right shape is a **chain of two DataLoaders**, batched at each layer:

```go
func (r *userResolver) LastViewedCardgroup(ctx context.Context, obj *model.User) (*model.Cardgroup, error) {
    loaders := loader.For(ctx)
    if loaders == nil {
        return nil, gqlerr.Internal(ctx, eris.New("loader: middleware not installed for /query"))
    }

    // Step 1: batch by user_id across every User in the response.
    pref, err := loaders.UserPreference.Load(ctx, obj.ID)()
    if err != nil { /* Cancelled / Internal */ }
    if pref == nil || pref.LastViewedCardgroupID == nil {
        return nil, nil
    }

    // Step 2: batch by cardgroup_id across every distinct preference target.
    cg, err := loaders.Cardgroup.Load(ctx, *pref.LastViewedCardgroupID)()
    if err != nil { /* ErrNotFound -> nil, Cancelled / Internal */ }
    return toCardgroupModel(cg), nil
}
```

For a response containing N users, the chain issues **two** batched queries total: one `SELECT * FROM user_preferences WHERE user_id IN (...)` and one `SELECT * FROM cardgroups WHERE id IN (...)`. Without the chain, a naive `for _, u := range users { repo.FindUserPreference(u.ID); repo.FindCardgroup(...) }` produces 2N queries. With a single loader that pre-joins, the resolver loses the layered cache because each cardgroup hit is also reachable through other resolvers (`Cardgroup.byId`, `Card.cardgroup`) and the per-request DataLoader for `Cardgroup` would already have it.

## Each loader stays scoped to its own aggregate

The `UserPreferenceLoader` returns `*domain.UserPreference`, never a hydrated `*domain.Cardgroup`. The resolver is the one place that knows both aggregates exist; the loader package stays free of cross-aggregate joins, so its batch function corresponds 1:1 with one repository method (`FindByUserIDs`). The chain composes at the resolver, not inside a "smart loader" that pre-joins — pre-joining would duplicate cardgroup hydration with the existing `CardgroupLoader` and double-fetch the same row whenever both fields appear in the same query.

## Missing-key semantics differ between layers — see the divergence note

The two loaders are **not symmetric** in how they report "no row found": the cardgroup loader returns `repository.ErrNotFound` (absence is a fetch failure for that aggregate); the user-preference loader returns `nil data` with `nil error` (absence is a legitimate "user has set no preferences yet" state). The resolver branches on both shapes — `pref == nil` short-circuits to `nil, nil`, and `errors.Is(err, repository.ErrNotFound)` on the cardgroup step also short-circuits to `nil, nil`. The full rule and the rationale live in [`docs/backend/library-gotchas/dataloader-missing-key-semantics.md`](dataloader-missing-key-semantics.md).

## Reference

`backend/graph/resolver/last_viewed_cardgroup.resolvers.go` — `userResolver.LastViewedCardgroup` is the canonical example. `backend/internal/loader/user_preference.go` carries the first-step loader; `backend/internal/loader/cardgroup.go` carries the second-step loader. Both registries are built per-request by the `loader.For(ctx)` middleware so the batch window matches the GraphQL request scope.
