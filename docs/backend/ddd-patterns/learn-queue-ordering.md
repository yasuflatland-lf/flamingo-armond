# Learn queue ordering

> Part of the [DDD patterns](../../../.claude/rules/ddd-patterns.md) rules.

## Problem

A permanent review backlog requires selecting cards whose memories can still be
maintained. Random selection lets freshly learned words disappear for weeks.
Selection and per-kind order must survive session arrangement.

## Policy

The repository selects and orders two independent windows: reviews by descending
FSRS retrievability, and never-seen cards newest-added first. `OrderingPolicy.Apply`
preserves both orders and interleaves at the user's `NewCardRatio` by largest
remainder. The default 1/5 ratio gives 4 new and 16 review cards in a full-pool
20-card session. Half-card ties favor new; when reviews exist, one- and two-card
sessions contain no new card, and the first new slot is 3. This remains inside
the `denominator - numerator` bound of slot 4. When a pool empties, the other
supplies the remainder. The usecase truncates to the requested session limit.

## Mechanics

| Stage | Owner | Behavior |
|---|---|---|
| Selection and per-kind order | `findDueCardsOn` | Independent review and new windows, each capped at `limit`. Reviews sort by `floor(extract(epoch FROM (Now - last_review)) / 86400) / greatest(stability, 0.001) ASC`, then `due ASC, cards.id ASC`. New cards sort by `cards.created_at DESC, cards.position DESC, cards.id DESC`. |
| Arrangement | `OrderingPolicy.Apply` | Partition by `Phase == FSRSPhaseNew`, preserving input order, then interleave at the ratio. |
| Truncation | `NextDueCards` | Cap the interleaved result with `ordered[:n]`. |

In go-fsrs v4.0.0, `parameters.go:ForgettingCurve` computes
`R = (1 + factor * t / S)^decay`; `arithmetic.go:decayAndFactor` supplies a positive
factor and negative decay. Thus R strictly decreases with t/S, making ascending
t/S equivalent to descending R without duplicating weights in SQL.
`steps.go:dateDiffRaw` floors elapsed hours divided by 24.
`arithmetic.go:constrainStability` clamps stability to [0.001, 36500]; the SQL
lower clamp prevents division by zero. `LearnWindow.Now` is bound into the order
expression so injected clocks and production use the same computation.

The new-card add instant is preserved by import upserts. Descending position
orders a same-instant batch toward the end of its source document, and ID breaks
ties. A reviewed card leaves the never-seen window.

## Contracts

- Review eligibility requires `ucs.due IS NOT NULL`, `ucs.due < window.DueBefore`,
  `ucs.last_review < window.ReviewedBefore`, and
  `ucs.last_review < window.CreditReviewedBefore`.
- `DueBefore` is `EndOfLearnDay`, the exclusive next JST midnight. Every review
  due later today is eligible; one due at or after midnight is excluded. This
  day-granular bound avoids a recurring time-of-day delay.
- `ReviewedBefore` is `StartOfLearnDay`. The strict bound excludes cards already
  reviewed in today's JST learn day and complements `ReviewedWithinLearnDay`.
- `CreditReviewedBefore` is UTC midnight of now's UTC date. The strict bound
  prevents a repeat earning zero scheduling credit under `EarnsSchedulingCredit`.
  Before 09:00 JST this is the tighter bound; afterward the JST cutoff is tighter.
  An overnight review can qualify in fewer than 24 hours.
- At most `2*limit` rows return, reviews first and new rows second. The policy
  preserves order within each kind, and the usecase owns the final cap.
- `LearnedStabilityDays` governs mastery tiers and statistics, not queue selection.

## Trade-off

With a permanent backlog, low-R cards wait while higher-R memories keep cycling.
Review throughput is the ratio's responsibility; order determines which cards
receive that capacity. This optimizes retained memories under a review cap.

The FSRS author's simulation used 20,000 cards, 20 new/day, an 80-review/day cap,
and desired retention 0.8. Its reported results were:

| Order | Average true retention | Seconds / remembered card |
|---|---|---|
| retrievability_desc | 0.798 | 85.4 |
| add_order_desc | 0.797 | 85.4 |
| retrievability_asc | 0.698 | 86.8 |
| random | 0.646 | 94.6 |
| due_date_asc | 0.605 | 96.8 |

## Reference

- [FSRS review sort-order simulation](https://github.com/open-spaced-repetition/review-sort-order-comparison), `notebook.ipynb` results table.
- [Improving sort orders](https://forums.ankiweb.net/t/improving-sort-orders/50081).
- `github.com/open-spaced-repetition/go-fsrs/v4@v4.0.0`: `ForgettingCurve`, `decayAndFactor`, `dateDiffRaw`, `constrainStability`.
- `backend/internal/domain/learn_day.go`: `NewLearnWindow`, `CreditReviewedBefore`.
- `backend/internal/repository/card_due.go`: `findDueCardsOn`.
- `backend/internal/domain/service/due_card_ordering.go`: `Apply`, `partition`, `interleave`.
