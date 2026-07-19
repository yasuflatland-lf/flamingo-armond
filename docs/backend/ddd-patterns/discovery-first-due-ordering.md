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

**80/20 is a full-pool target, not an invariant.** When both candidate pools are
full — the deck has at least 16 never-seen cards *and* at least 4 eligible
prior-day reviews — a default 20-card session composes as **16 uniformly-sampled
never-seen cards (80%)** interleaved with **4 prior-day review cards (20%)**
(16/4 at `n = 20` under `domain.DefaultNewCardRatio` = 4/5). That 16/4 split is
the composition *only under full pools*; the mechanics below degrade it toward
review whenever either pool runs short. The skew is deliberate, not a bug:
review cards are due and time-critical (skipping them decays memory and FSRS
scheduling), while new cards are discretionary (deferrable at no cost), so
favoring review under a shallow pool is correct spaced-repetition behavior.

Three mechanisms shape the actual mix, each biased toward review:

- **Review-first per-cycle emission.** `interleave` emits `ratio.ReviewShare()`
  review cards *before* `ratio.NewShare()` new cards on every cycle. At the 4/5
  default that is 1 review then 4 new per cycle (`OrderingPolicy.Apply` →
  `interleave` in `due_card_ordering.go`).
- **Review-favoring rounding on non-divisible sizes.** When the session limit is
  not a multiple of the ratio denominator, the trailing partial cycle emits its
  review portion first, so the review count rounds *up* by at most one
  `ReviewShare` rather than down. A full-pool request for 22 cards under 4/5
  yields 5 review + 17 new (not 4 review + 18 new).
- **Drain back-fill toward review.** The rescue-review, filler-review, and new
  windows are independent `LIMIT` selections that together return up to
  `3*limit` rows; the caller then truncates the interleaved result to the session limit (`ordered[:n]`
  in `LearnUsecase.NextDueCards`). As the unseen pool empties, the new bucket
  runs out after its early contributions and `interleave` appends the remaining
  review cards, so the session skews toward review. Near deck completion, with a
  single never-seen card left, a 20-card session becomes 1 new + 19 review — the
  session is review-heavy because there is almost nothing left to discover.

Within those mechanisms:

- **Review slots** take rescue-band cards first — rows whose latest rating was
  Again or whose FSRS stability is below `domain.LearnedStabilityDays`. The
  rescue window admits those cards when their due timestamp falls before the
  current JST learn day's exclusive end, so a rescue due later today can be
  served early. Rows outside the rescue band act as filler only after their due
  timestamp has arrived. A mature card last rated Hard is filler, not rescue.
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
| Selection (which rows enter each window) | `repository.FindDueCardsForUser` | Three independent `LIMIT` windows, each ordered by `random()`: rescue reviews (`due < rescueDueBefore AND last_review < reviewedBefore` plus `last_rating = Again OR stability < LearnedStabilityDays`), disjoint filler reviews (`due <= now` plus the inverse band predicate), then new cards with no FSRS row. |
| Arrangement (order within the batch) | `service.OrderingPolicy.Apply` | Injected `*rand.Rand` shuffles the new partition fully and the review partition within same-band runs; then interleaves at the caller-supplied ratio (`domain.DefaultNewCardRatio` = 4:1 absent a stored preference) with review-first emission. |
| Truncation | `usecase.LearnUsecase.NextDueCards` | Caps the interleaved result to the session limit (`ordered[:n]`). Because the three windows return up to `3*limit` rows, this truncate is load-bearing: it yields the 16/4 split for a 20-card request only when both the combined review pool and new-card pool are full, and skews toward review when the unseen pool is short (see Policy). |

`random()` runs in Postgres and cannot be seeded from Go, so it decides only
*which* rows are eligible; the deterministic arrangement is the injected
`*rand.Rand`'s job (see [inject `*rand.Rand` into pure functions](../library-gotchas/inject-rand-rand-for-deterministic-test.md)).

## Contracts

- **Repository concatenates rescue reviews before filler reviews.** The rescue
  and filler predicates are disjoint, and each window uses its own `LIMIT` and
  `ORDER BY random()`. This is a contract with `OrderingPolicy`'s
  `shuffleWithinBand`, which detects each contiguous band with a single linear
  pass and never shuffles across the boundary. A filler card therefore cannot
  displace a rescue card from the review slots. The filler predicate uses
  `last_rating IS DISTINCT FROM Again`, so a backfilled NULL rating with learned
  stability cannot disappear through SQL three-valued logic.
- **The learn-day boundaries are fixed UTC+9 instants.**
  `domain.StartOfLearnDay(now)` computes the learner's JST midnight at or before
  now, and `domain.EndOfLearnDay(now)` adds exactly 24 hours to obtain the
  following midnight. JST observes no daylight saving, so the fixed offset and
  addition are exact and avoid a tzdata dependency. The values are passed as
  instants and compared directly with UTC-stored timestamps. The product
  currently assumes a Japan-resident learner; revisit with a per-user timezone
  preference if that assumption breaks.
- **The rescue due cutoff is strictly before learn-day end.** The rescue
  predicate uses `ucs.due < rescueDueBefore`, where `rescueDueBefore` is
  `domain.EndOfLearnDay(now)`. A rescue due later today is eligible, while one
  due exactly at the next JST midnight is not. Filler remains time-granular and
  uses `ucs.due <= now`.
- **The review-window cutoff is strictly before the boundary.** The repository
  predicate is `ucs.last_review < ?` (strict `<`), so a card whose `last_review`
  equals the JST start-of-day exactly is excluded — a card swiped at local
  midnight does not reappear in today's queue. The `<` vs `<=` choice is part of
  the contract and is pinned by an exact-boundary fixture; see
  [exact-boundary fixture for strict time-cutoff predicates](../library-gotchas/strict-cutoff-boundary-fixture-and-mutation-proof.md).

## Trade-off

Discovery is bought at the cost of review efficiency. Rescue-band cards claim
the review slots before filler reviews, and rescue cards due later in the JST
day may be served before their exact due time. A large filler backlog therefore
drains more slowly than a pure due-date order would drain it. This is deliberate:
queue's primary job became surfacing the unseen backlog, not maximising
retention throughput. `cards.position` remains Notion-sync metadata (assigned
as the zero-based document index, overwritten on re-sync) but no longer drives
learn ordering — new cards are sampled randomly, not walked in document order.

## Reference

- `backend/internal/domain/learn_day.go` — `StartOfLearnDay`, `EndOfLearnDay`.
- `backend/internal/domain/mastery_tier.go` — `LearnedStabilityDays`.
- `backend/internal/domain/due_card.go` — `DueCard.Rescue`.
- `backend/internal/domain/service/due_card_ordering.go` — `OrderingPolicy.Apply`,
  `shuffleWithinBand`, `interleave` (new/review shares supplied by the caller's ratio).
- `backend/internal/domain/new_card_ratio.go` — `NewCardRatio` VO, `DefaultNewCardRatio` (4/5, the default 4:1 interleave).
- `backend/internal/repository/card_due.go` — `FindDueCardsForUser`,
  `findDueCardsOn`, `dueRowsOn` (the three-window selection).
- `backend/internal/usecase/learn.go` — `LearnUsecase.NextDueCards`
  (interleave invocation and per-session truncation).
