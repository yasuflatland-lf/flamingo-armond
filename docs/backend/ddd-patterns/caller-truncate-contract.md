# Caller-truncate contract: domain service returns all, callers cap

> Part of the [DDD patterns](../../../.claude/rules/ddd-patterns.md) rules.

## Why

A domain service that applies session-local ordering (shuffle, interleave,
priority weighting) operates on a slice of candidate cards already filtered
and capped at the repository boundary. The service does not know the
per-session limit a particular caller wants to enforce — different transports
(`learnNextDueCards` query, `handleSwipe` mutation) may want different caps,
or the same transport may pass a user-supplied `limit` that is lower than the
repository's request size.

Two valid placements:

1. **Service truncates** — the domain service takes a `limit` argument and
   returns at most `limit` cards.
2. **Caller truncates** — the domain service returns all cards it processed;
   each caller slices the result down to its own limit.

The codebase picks (2) because the ordering policy's intermediate shape is
pure (no length cap), and the limit is a presentation/usecase concern that
varies across call sites. The trade-off is that every caller must remember
to truncate.

## What

`OrderingPolicy.Apply` returns all cards it received, in the new
order:

```go
// The caller is responsible for truncating to a per-session limit. Apply
// returns all cards from due without imposing a length cap; the input slice
// is not modified, as partition produces fresh slices before shuffling.
func (p *OrderingPolicy) Apply(due []domain.DueCard, rng *rand.Rand) []*domain.Card { ... }
```

Every caller truncates with the same shape immediately after:

```go
// backend/internal/usecase/learn.go
ordered := u.ordering.Apply(due, u.randSource())
if len(ordered) > n {
    ordered = ordered[:n]
}

// backend/internal/usecase/swipe.go
nextCards = u.ordering.Apply(due, u.randSource())
if len(nextCards) > u.nextBatchSize {
    nextCards = nextCards[:u.nextBatchSize]
}
```

## Failure mode and mitigations

The risk is a third caller forgetting the truncate, in which case the
service returns more rows than the caller's contract allows. Three things
keep that from drifting:

1. **The docstring on `Apply` names the responsibility explicitly.** A
   reviewer reading the call site can grep the service signature and see
   "the caller is responsible for truncating".
2. **The repository already caps at the caller-requested limit.** The
   service therefore receives at most `limit` rows; the truncate after
   `Apply` is a redundant guard for the case where the ordering policy
   would otherwise emit a longer slice (it does not today, but the guard
   future-proofs against changes that interleave additional sources).
3. **Direct unit tests cover the post-truncate shape.** Each caller's
   tests assert the final slice length, so a missing truncate surfaces at
   review time, not in production.

If a future ordering policy can legitimately emit more cards than it
received (e.g. interleaving a separate "boosted" bucket), the truncate
becomes load-bearing and the contract is unchanged: caller still owns the
cap. If every call site grows to do the same truncate immediately and the
limit becomes a session-wide config, only then is the contract a candidate
to flip to service-truncates.

## Reference

- `backend/internal/domain/service/due_card_ordering.go` — `Apply`
  docstring states the caller-truncate contract.
- `backend/internal/usecase/learn.go` and `backend/internal/usecase/swipe.go`
  — both callers truncate after `Apply`. A grep for `ordering.Apply` returns
  exactly these two sites today; either one missing the truncate is a
  review-time defect.
