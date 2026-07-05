# Discovery-first due ordering (80% new / 20% prior-day review)

> Part of the [DDD patterns](../../../.claude/rules/ddd-patterns.md) rules.

## Problem

An earlier learn-queue ordering layered a deterministic tiebreaker (Notion
document order via `cards.position`) under the FSRS due key and shuffled only
**within ties** — equal `position` runs for new cards, equal `due` runs for
review cards. The tie-scoped shuffles never fired on real data: production
`cards.position` values are distinct per Notion sync, and `user_card_fsrs.due`
carries microsecond precision, so two rows almost never tie on either key.
A 1258-card cardgroup produced fully deterministic batches dominated by
overdue learning cards; with an old 4-reviews-to-1-new interleave, 1000+
never-seen cards barely surfaced. Tie-only randomness is structurally dead
whenever the keys it scopes to are dense.

## Policy

A default 20-card session is **16 uniformly-sampled never-seen cards (80%)**
interleaved with **4 prior-day review cards (20%)**:

- **Review slots** take learning-phase cards first — those in
  `FSRSStateLearning` or `FSRSStateRelearning`, i.e. whose latest rating was
  Again or Hard (`domain.FSRSCardState.IsLearningPhase()`). Long-interval
  `FSRSStateReview` cards act as filler when fewer than four learning-phase
  cards are due.
- A card whose `last_review` is at or after the learner's JST start-of-today is
  **excluded** from the review window, so a card swiped today never reappears
  in today's queue regardless of its FSRS re-due interval.
- New-card sampling is uniform across the whole unseen pool via SQL `random()`,
  so consecutive sessions surface different cards instead of walking the
  deterministic `created_at` / `position` (document) order.

## Mechanics

Selection and arrangement are deliberately split across layers so tests stay
deterministic while the database does the sampling:

| Stage | Owner | Behaviour |
|---|---|---|
| Selection (which rows enter each window) | `repository.FindDueCardsForUser` | Two independent `LIMIT` windows: a review window (`due <= now AND last_review < reviewedBefore`, ordered learning-phase-first via a `CASE` then `random()`) concatenated with a new window (no FSRS row, ordered by `random()`). |
| Arrangement (order within the batch) | `service.OrderingPolicy.Apply` | Injected `*rand.Rand` shuffles the new partition fully and the review partition within same-phase runs; then interleaves at the caller-supplied ratio (`domain.DefaultNewCardRatio` = 4:1 absent a stored preference) with review-first emission. |
| Truncation | `usecase.LearnUsecase.NextDueCards` | Caps the interleaved result to the session limit (`ordered[:n]`), yielding the 16/4 split for a 20-card request. |

`random()` runs in Postgres and cannot be seeded from Go, so it decides only
*which* rows are eligible; the deterministic arrangement is the injected
`*rand.Rand`'s job (see [inject `*rand.Rand` into pure functions](../library-gotchas/inject-rand-rand-for-deterministic-test.md)).

## Contracts

- **Repository pre-sorts review rows learning-phase-first.** The review
  window's `ORDER BY CASE WHEN ucs.state IN (Learning, Relearning) THEN 0 ELSE 1
  END, random()` is a contract with `OrderingPolicy`'s `shuffleWithinPhase`,
  which detects each phase run with a single linear pass and never shuffles
  across the boundary. A Review-state filler card therefore cannot displace a
  learning-phase card from the review slots.
- **`startOfDayJST` is a fixed UTC+9 boundary.** `usecase.startOfDayJST(now)`
  computes the learner's local midnight with `time.FixedZone("JST", 9*60*60)`.
  JST observes no daylight saving, so a fixed offset is exact and avoids a
  tzdata dependency. The product currently assumes a Japan-resident learner;
  revisit with a per-user timezone preference if that assumption breaks.
- **The review-window cutoff is strictly before the boundary.** The repository
  predicate is `ucs.last_review < ?` (strict `<`), so a card whose `last_review`
  equals the JST start-of-day exactly is excluded — a card swiped at local
  midnight does not reappear in today's queue. The `<` vs `<=` choice is part of
  the contract and is pinned by an exact-boundary fixture; see
  [exact-boundary fixture for strict time-cutoff predicates](../library-gotchas/strict-cutoff-boundary-fixture-and-mutation-proof.md).

## Trade-off

Discovery is bought at the cost of review efficiency. Long-interval
`FSRSStateReview` cards (last rated Easy/Good) compete for the same four review
slots per batch as learning-phase cards, so a large Review backlog drains more
slowly than a pure due-date order would drain it. This is deliberate: the
queue's primary job became surfacing the unseen backlog, not maximising
retention throughput. `cards.position` remains Notion-sync metadata (assigned
as the zero-based document index, overwritten on re-sync) but no longer drives
learn ordering — new cards are sampled randomly, not walked in document order.

## Reference

- `backend/internal/domain/fsrs_state.go` — `FSRSCardState.IsLearningPhase()`.
- `backend/internal/domain/service/due_card_ordering.go` — `OrderingPolicy.Apply`,
  `shuffleWithinPhase`, `interleave` (new/review shares supplied by the caller's ratio).
- `backend/internal/domain/new_card_ratio.go` — `NewCardRatio` VO, `DefaultNewCardRatio` (4/5, the default 4:1 interleave).
- `backend/internal/repository/card.go` — `FindDueCardsForUser`, `findDueCardsOn`,
  `dueRowsOn` (the two-window selection).
- `backend/internal/usecase/learn.go` — `startOfDayJST`, `LearnUsecase.NextDueCards`
  (interleave invocation and per-session truncation).
