# DataLoader missing-key semantics: `nil data` vs `ErrNotFound` is per-aggregate

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

Two `dataloader/v7` batch functions in this codebase encode opposite postures for "no row matched this key", and the difference is deliberate:

| Loader | Missing-key result | Rationale |
|---|---|---|
| `CardgroupLoader` | `&Result{Error: eris.Wrapf(repository.ErrNotFound, "cardgroup %s", k)}` | A cardgroup ID that resolves to nothing is a fetch failure for that aggregate — the caller asked for a specific row and the row does not exist. The resolver branches on `errors.Is(err, ErrNotFound)` to translate to a nullable response. |
| `UserPreferenceLoader` | `&Result{Data: nil}` (no error) | A user with no preference row is a legitimate domain state — "the user has not set any preferences yet". An error here would pollute the resolver with a per-request false-positive on every anonymous-leaning workload. |

Both shapes are correct **for their aggregate**. The wrong move is to force the two loaders into a uniform posture for the sake of consistency — that flattens away the domain-level distinction. Pick the posture that matches the aggregate's lifecycle:

- **"absence = error"** when the caller supplies a primary-key identifier they previously read from elsewhere (cardgroup ID embedded in a connection edge, an authenticated user ID, a referenced FK column). The reasonable client expectation is that the row exists; a miss is a stale-reference bug.
- **"absence = data state"** when the aggregate is conditionally created (lazy-upsert preference rows, per-user audit bookmarks). The reasonable client expectation is that the row may or may not exist; the resolver must branch on `nil`.

## Comment the choice at the batch function

Drop a one-line rationale next to the divergent branch so future maintainers do not flatten the two postures:

```go
// Missing keys yield nil data with nil error: absence means "no preference
// set yet", not a fetch failure. Callers branch on pref == nil.
for i, k := range keys {
    out[i] = &dataloader.Result[*domain.UserPreference]{Data: byUserID[k]}
}
```

The comment is the load-bearing artefact when the next reader is comparing the two loaders and trying to decide which shape to copy. A silent `Data: byUserID[k]` (which evaluates to `nil` when the key is missing) reads as a bug until the comment names it as deliberate.

## Resolver-side branching is asymmetric

The two-step chain documented in [DataLoader two-step chain](dataloader-two-step-chain.md) shows the asymmetry concretely:

```go
pref, err := loaders.UserPreference.Load(ctx, obj.ID)()
if err != nil { /* Cancelled / Internal — never ErrNotFound */ }
if pref == nil || pref.LastViewedCardgroupID == nil {
    return nil, nil  // legitimate empty state
}

cg, err := loaders.Cardgroup.Load(ctx, *pref.LastViewedCardgroupID)()
if err != nil {
    if errors.Is(err, repository.ErrNotFound) {
        return nil, nil  // race window: SET NULL has not propagated yet
    }
    /* Cancelled / Internal */
}
```

The cardgroup step **must** check `errors.Is(err, ErrNotFound)` explicitly; the user-preference step **must not** — there is no such error there. A resolver that copies the cardgroup branch over to the preference step adds dead code; a resolver that copies the preference branch over to the cardgroup step silently turns a stale FK reference into a `nil` response with no log entry.

## Reference

`backend/internal/loader/cardgroup.go` returns `eris.Wrapf(repository.ErrNotFound, "cardgroup %s", k)` for missing keys. `backend/internal/loader/user_preference.go` returns `&Result{Data: byUserID[k]}` with `byUserID` lookup yielding `nil` when the key is absent. `backend/graph/resolver/last_viewed_cardgroup.resolvers.go` — `userResolver.LastViewedCardgroup` is the canonical consumer that branches on both shapes.
