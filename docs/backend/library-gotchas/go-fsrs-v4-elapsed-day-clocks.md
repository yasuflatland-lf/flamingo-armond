# go-fsrs v4's two elapsed-day clocks

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

## The two formulas

**`steps.go` defines two non-equivalent elapsed-day formulas.** In
go-fsrs/v4 v4.0.0, their implementations are:

```go
func dateDiffInDays(last, cur time.Time) uint64 {
	lr := last.UTC()
	utc1 := time.Date(lr.Year(), lr.Month(), lr.Day(), 0, 0, 0, 0, time.UTC)
	n := cur.UTC()
	utc2 := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
	hours := utc2.Sub(utc1).Hours()
	if hours < 0 {
		return 0
	}
	return uint64(math.Floor(hours / 24))
}

func dateDiffRaw(last, cur time.Time) float64 {
	return math.Floor(cur.Sub(last).Hours() / 24)
}
```

`Scheduler.elapsedDays` in `scheduler.go` calls `dateDiffInDays` for the
review-scheduling path reached from `FSRS.Next` and `FSRS.Repeat` in `fsrs.go`.
Both operands are truncated to UTC midnight, so the result is a calendar-date
difference. `FSRS.Retrievability` in `fsrs.go` is the only production caller of
`dateDiffRaw`; it floors the untruncated wall-clock duration.

## Measured divergence

**A one-hour gap that crosses UTC midnight is the trap.** Replaying
`FSRS.Next` through `Scheduler.elapsedDays` and comparing its input with
`FSRS.Retrievability` at v4.0.0 gives:

| gap | UTC dates | scheduler credit | `Retrievability`'s elapsed |
|---|---|---|---|
| 23h (09:00 → next 08:00 JST) | same | **0 days** — stability change `+0.0000` | 0 days |
| 1h (08:30 → 09:30 JST) | different | **1 day** — stability `+1.9095` | 0 days |
| 24h | different | 1 day — stability `+1.9095` | 1 day |

The middle row makes `FSRS.Retrievability` in `fsrs.go` report perfect
retention while `longTermScheduler.reviewState` in `scheduler_longterm.go`
has already passed one full calendar day to `Parameters.ForgettingCurve` and
applied the resulting stability change.

## The Z3 relationship

**The mismatch is bounded and predictable for ordered reviews.** Z3 proves the
relationship between `dateDiffInDays` and `dateDiffRaw` in `steps.go` exactly:

```text
0 <= calendar - raw <= 1
```

The difference is 1 if and only if the review's UTC time-of-day precedes the
previous review's UTC time-of-day; otherwise it is 0. This follows from
`dateDiffInDays` discarding both times-of-day before subtraction while
`dateDiffRaw` retains them until after subtraction.

## Adopting `FSRS.Retrievability`

**A retrievability value must carry the clock that produced it.**
`FSRS.Retrievability` in `fsrs.go` must not be presented next to a due date or
a scheduling decision without stating that it uses `dateDiffRaw`'s wall-clock
days. It must never be used to reconstruct what `FSRS.Next` or `FSRS.Repeat`
did: those methods reach `Scheduler.elapsedDays` in `scheduler.go`, whose
calendar-date input can differ by one day.

## A zero-credit repeat is not neutral

**`Scheduler.elapsedDays == 0` does not make `FSRS.Next` a no-op.** With
`S = 6.9`, `D = 5.0`, and reviews 12 hours apart inside one UTC date,
`longTermScheduler.reviewState` produces these measured results:

- `Again` gives `S 6.9000 → 0.9701` and `D 5.00 → 8.34`, with the due date
  collapsing from 7 days to 1.
- `Hard` and `Easy` leave stability bit-exact while moving difficulty by
  `+1.67` and `-1.69`, respectively.

Across a grid of stabilities and difficulties, the maximum `|ΔS|` for
`Hard`, `Good`, and `Easy` is `0.000e+00`. The exact recall-rating result comes
from `Parameters.nextRecallStability` in `arithmetic.go`: at
`retrievability = 1`, its `exp((1-r)*W10)-1` term is bit-exactly 0.
`Parameters.nextDifficulty`, called alongside it by `Scheduler.nextDs` in
`scheduler.go`, has no retrievability term at all, so difficulty still moves on
every rating despite zero scheduling credit. `Again` instead uses
`Parameters.nextForgetStability`, which accounts for the stability collapse.

This non-neutral mutation is why `RescueReviewedBefore` in
[`backend/internal/domain/learn_day.go`](../../../backend/internal/domain/learn_day.go)
bounds the serving windows at UTC midnight of the current UTC calendar date
rather than treating zero scheduling credit as a safe repeat.

## The `interval == stability` identity

**The raw interval conversion is the identity only when
`RequestRetention == 0.9`.** `stabilityToInterval` and
`Parameters.decayAndFactor` in `arithmetic.go` define:

```text
stabilityToInterval(s, R) = s / factor * (R^(1/decay) - 1)
factor = 0.9^(1/decay) - 1
```

At `R = 0.9`, the factor cancels for any decay, so the raw interval equals
stability exactly. Z3 proves the identity, and a numeric replay over
`{0.001, 0.5, 6.9, 7.0, 20.5, 21.0, 36500}` finds no exception.
`Parameters.nextInterval` in `arithmetic.go` applies `constrainStability` to
its stability argument before this raw conversion, then rounding, interval
bounds, and optional fuzz after it.

`DefaultParam` in `parameters.go` sets `RequestRetention` to `0.9`, and
`NewFSRSScheduler` in
[`backend/internal/domain/service/fsrs_scheduler.go`](../../../backend/internal/domain/service/fsrs_scheduler.go)
changes only `EnableShortTerm`. Under that shipped retention,
[`domain.MatureStabilityDays = 21`](../../../backend/internal/domain/mastery_tier.go)
therefore reads the same on Anki's interval scale and FSRS's stability scale.
For any other retention, the numerator no longer equals `factor`, and the
identity breaks.

## Reference

- [`backend/go.mod`](../../../backend/go.mod) — the go-fsrs/v4 v4.0.0 pin.
- [`steps.go`](https://github.com/open-spaced-repetition/go-fsrs/blob/v4.0.0/steps.go) —
  `dateDiffInDays` and `dateDiffRaw`.
- [`scheduler.go`](https://github.com/open-spaced-repetition/go-fsrs/blob/v4.0.0/scheduler.go) —
  `Scheduler.elapsedDays` and `Scheduler.nextDs`.
- [`fsrs.go`](https://github.com/open-spaced-repetition/go-fsrs/blob/v4.0.0/fsrs.go) —
  `FSRS.Next`, `FSRS.Repeat`, and `FSRS.Retrievability`.
- [`scheduler_longterm.go`](https://github.com/open-spaced-repetition/go-fsrs/blob/v4.0.0/scheduler_longterm.go) —
  `longTermScheduler.reviewState` and `longTermScheduler.nextInterval`.
- [`arithmetic.go`](https://github.com/open-spaced-repetition/go-fsrs/blob/v4.0.0/arithmetic.go) —
  `constrainStability`, `Parameters.decayAndFactor`,
  `stabilityToInterval`, `Parameters.nextInterval`,
  `Parameters.nextDifficulty`, `Parameters.nextRecallStability`, and
  `Parameters.nextForgetStability`.
- [`parameters.go`](https://github.com/open-spaced-repetition/go-fsrs/blob/v4.0.0/parameters.go) —
  `DefaultParam` and `Parameters.ForgettingCurve`.
- [`backend/internal/domain/service/fsrs_scheduler.go`](../../../backend/internal/domain/service/fsrs_scheduler.go) —
  `NewFSRSScheduler`.
- [`backend/internal/domain/learn_day.go`](../../../backend/internal/domain/learn_day.go) —
  `RescueReviewedBefore` and `utcCalendarDay`.
- [`backend/internal/domain/mastery_tier.go`](../../../backend/internal/domain/mastery_tier.go) —
  `MatureStabilityDays`.
