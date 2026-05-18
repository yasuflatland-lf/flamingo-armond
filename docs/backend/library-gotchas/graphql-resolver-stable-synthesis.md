# GraphQL resolver: synthesized domain values must be deterministic

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

A GraphQL resolver sometimes needs to synthesize a domain value for rows that have no corresponding record yet. The synthesized value must be deterministic — it must produce the same result on every call for the same inputs — so it stays consistent with the SQL ordering key used to sort those rows.

**The failure mode:** using `time.Now()` to synthesize a "new card" state generates a different `due` timestamp on every page load. If the SQL query orders by `COALESCE(ucs.due, cards.created_at)`, the resolver's synthesized `due` diverges from the SQL ordering key, and the client sees a card at position X in one query and position Y in the next.

```go
// Wrong: time.Now() differs between requests.
if ucs == nil {
    ucs = domain.NewUserCardFSRSForNewCard(user.Sub, obj.ID, time.Now().UTC())
}

// Correct: obj.CreatedAt matches COALESCE(ucs.due, cards.created_at) in SQL.
if ucs == nil {
    ucs = domain.NewUserCardFSRSForNewCard(user.Sub, obj.ID, obj.CreatedAt)
}
```

**Rule:** when a resolver synthesizes a domain value for a row with no persisted state, derive the synthesis input from a field already present on the parent object (`obj.CreatedAt`, `obj.ID`, etc.), not from wall-clock time. The field chosen must match the SQL `COALESCE` fallback so resolver output and SQL ordering agree.

**Applies to:** any resolver that lazy-initialises a missing aggregate record and returns a field that participates in pagination or ordering. Loader-backed resolvers are the most common pattern — when `loaders.UserCardFSRS.Load(ctx, obj.ID)()` returns `nil`, the nil check triggers synthesis. Whatever timestamp appears in `COALESCE(persisted_col, fallback_col)` in the ordering query, the synthesis must use the same fallback field.

Reference: `backend/graph/resolver/card.resolvers.go` — `cardResolver.UserCardState` synthesises a new-card state from `obj.CreatedAt`; `backend/internal/domain/user_card_fsrs.go` — `NewUserCardFSRSForNewCard(userID, cardID string, now time.Time)`.
