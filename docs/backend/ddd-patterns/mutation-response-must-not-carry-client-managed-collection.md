# Mutation response must not carry a client-managed collection

> Part of the [DDD patterns](../../../.claude/rules/ddd-patterns.md) rules.

## Why

A write mutation records a fact (a swipe rating, a role grant, a card update).
When its response also includes a freshly-computed snapshot of a collection
that the client already manages — a swipe queue, a search result list, paginated
cards — two systems become authoritative for the same conceptual collection
simultaneously. That dual-source-of-truth arrangement breaks in at least two ways:

1. **Non-deterministic server computation.** A snapshot seeded by `time.Now()`
   (shuffle, interleave, priority weighting) produces a different ordering on
   every request for the same logical set. If the client replaces its in-memory
   collection with the server's snapshot, the visible order changes on each
   mutation response — a reshuffle the user did not ask for.

2. **Ignored dead bytes.** If the client ignores the snapshot to preserve its
   own order, the field travels across the wire and is deserialized for nothing.
   The server's snapshot is never applied, which means it was never part of the
   contract in a meaningful sense.

Either way, the snapshot in the mutation response is wrong from a contract
perspective. The server cannot know which subset of the collection the client
already holds, what order the client chose, or what the client has already
consumed. Embedding a collection snapshot in a write response couples the
write and read lifecycles in a way the client cannot reliably consume.

## What

Write mutations should return only data that is scoped to the write itself:

- The affected entity (the updated card, the new role assignment).
- The affected entity's post-write state — a single record, not a collection.
  `SwipeResponse.userCardState: UserCardState!` carries the reviewed card's
  FSRS scheduling state *after* the swipe was applied. This is the positive
  complement of the rule: one affected-entity snapshot lets the client render a
  before/after delta (e.g. a "Memory +N%" badge) with no refetch, because the
  client already holds the pre-write value from the row it acted on. A
  *collection* snapshot fails for the dual-source-of-truth reasons above; a
  single affected-entity record does not.
- An outcome enum or union (`SwipeSuccess`, `ValidationError`, `Forbidden`).
- Telemetry counts or timestamps belonging to the write (the FSRS rating
  stored, the timestamp recorded).
- Field-level validation errors.

Collection refills are a separate concern. The client triggers a prefetch or
pagination query with its own caching and lifecycle. The collection is a client
concern; the server exposes a source-of-truth query — not a snapshot embedded
in an unrelated response.

**Server — before (anti-pattern):**

```go
// SwipeResponse carries a freshly-shuffled snapshot on every write.
type SwipeResponse struct {
    UserCardFSRS *model.UserCardFSRS
    NextCards    []*model.Card // dual source of truth
}
```

**Server — after:**

```go
// SwipeResponse is scoped to the write only.
type SwipeResponse struct {
    UserCardFSRS *model.UserCardFSRS
}
```

**Client — before (anti-pattern):**

```ts
// Replaces the queue on every mutation response — visible reshuffle.
const [queue, setQueue] = useState<Card[]>(initialCards);

onSwipeComplete(payload) {
    setQueue(payload.nextCards); // overwrites client order with server snapshot
}
```

**Client — after:**

```ts
// Client owns the queue. Mutation only advances (removes the swiped card).
const [queue, setQueue] = useState<Card[]>(initialCards);

onSwipeComplete(swipedCardId) {
    setQueue(prev => prev.filter(c => c.id !== swipedCardId));
    // Prefetch (learnNextDueCards) replenishes when queue runs low — separate lifecycle.
}
```

## Check before adding a field to a mutation response

Ask these questions in order:

| Question | Answer → action |
|---|---|
| Is the field scoped to this write? (affected entity, outcome enum, telemetry, validation error) | Yes → include it. |
| Is the field the affected entity's post-write state — a single record, not a collection? (e.g. `userCardState`) | Yes → include it; the client can render a before/after delta with no refetch. |
| Is the field a collection the client already manages? (queue, list, search results, pagination) | Yes → exclude it. Provide a separate query. |
| Does the field's computation involve `time.Now()`, random state, or any non-deterministic input? | Yes → strong signal it does not belong in a write response; exclude it. |

## Worked example — issue #229

Before the fix, `handleSwipe` returned `nextCards`: a freshly-shuffled slice
of all due cards. The frontend's `onSwipeComplete` called
`setQueue(payload.response.nextCards)`, replacing the entire queue on every
swipe. Because `OrderingPolicy.Apply` uses a `time.Now()`-seeded random source,
each response carried a different ordering for the same logical set, causing a
visible reshuffle of the remaining cards after every swipe.

The fix dropped `nextCards` from `SwipeResponse`. The frontend's existing
`learnNextDueCards` prefetch became the sole refill mechanism; the optimistic
delete (`setQueue(prev => prev.filter(...))`) became the sole queue advance.
Queue order is now stable across mutations — no reshuffle, no dead bytes in
the response.

## Cross-references

- [`.claude/rules/pagination.md`](../../../.claude/rules/pagination.md) — Connection
  patterns apply the same separation of concerns to paginated lists: the server
  exposes a source-of-truth query; clients do not treat mutation side-channels
  as authoritative list state.
- [`docs/backend/error-wrapping/result-union-errors-as-data.md`](../error-wrapping/result-union-errors-as-data.md) —
  the contract-narrowing approach for what belongs in a mutation response
  (outcome unions, validation errors) vs. what belongs in a separate query.
