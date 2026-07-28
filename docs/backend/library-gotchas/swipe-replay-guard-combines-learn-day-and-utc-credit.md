# `swipeUsecase` uses the `Clock` port and combines both replay rules

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

The swipe path now uses the same injected time port as the other time-sensitive
usecases, and its replay guard enforces both the product's JST learn-day spacing
rule and go-fsrs's UTC-calendar-date scheduling-credit rule. The two predicates
overlap, but neither subsumes the other.

## `swipeUsecase` holds the `Clock` port

`swipeUsecase` holds `Clock`, defaults it to `systemClock{}`, and reads
`u.clock.Now().UTC()` inside the transaction immediately before loading the
user's FSRS row. The public constructors retain their existing signatures.
In-package tests inject `fixedClock` through the concrete usecase field, which
lets boundary fixtures choose one reproducible instant.

`TestSwipeUsecase_HandleSwipe_SameLearnDayRepeat_IsSuccessShapedNoOp` and
`TestSwipeUsecase_HandleSwipe_LearnDayBoundary` in
[`backend/internal/usecase/swipe_learn_day_test.go`](../../../backend/internal/usecase/swipe_learn_day_test.go)
now use fixed instants. Fixture construction and the usecase read therefore
cannot land on opposite sides of the 15:00 UTC learn-day boundary.

The earlier TLA+ clock-skew check remains useful evidence about the old
deviation: with `MinInterval = 24` and the swipe read allowed to lead the serve
read by one hour, `NoZeroCreditReview` held across 5,924 distinct states at
depth 57. Injection removes the testability defect rather than changing that
result.

## The replay guard is a union, not a substitute

The recording path accepts a repeat as a success-shaped no-op when either rule
says the card must not be reviewed:

```go
// backend/internal/usecase/swipe.go
if existing != nil &&
	(domain.ReviewedWithinLearnDay(existing.State.LastReview, now) ||
		!domain.EarnsSchedulingCredit(existing.State.LastReview, now)) {
	// ... structured log of the ignored repeat ...
	return nil
}
```

`ReviewedWithinLearnDay` preserves the queue's product rule: a card swiped
today does not return during the same JST learn day. It is the exact complement
of the serving-side SQL `last_review < StartOfLearnDay(now)` window, so those
comparators must move together. It also blocks a nine-hour repeat from 16:00 UTC
(01:00 JST) to 01:00 UTC on the next UTC date: go-fsrs grants scheduling credit
because UTC midnight was crossed, but both instants remain in one JST learn day.

`EarnsSchedulingCredit` mirrors go-fsrs v4's `dateDiffInDays`: both operands are
reduced to UTC midnight and compared. A zero `LastReview` and a backward clock
step earn nothing. Wall-clock duration is not the rule: a one-hour gap across
UTC midnight earns the same stability increment as a 24-hour gap, while a
23-hour gap inside one UTC date earns none.

The old learn-day-only guard therefore left a hole of up to 23 hours, not merely
a one-hour midnight edge. JST midnight is 15:00 UTC. A review at 00:30 UTC
(09:30 JST) followed by a direct swipe at 23:30 UTC (08:30 JST the next day)
crosses the JST learn-day boundary but remains on one UTC date. `HandleSwipe`
accepts any owned card ID, so it cannot assume the card came from the queue.

The TLA+ results separate the two entry paths:

- Queue-only entry holds `NoZeroCreditReview` across 22,121 distinct states at
  depth 63.
- Direct swipe entry violates it at depth 18 when the replay guard is only
  learn-day granular.
- The earlier long-term configuration generated 6,278 states, 5,485 distinct
  states, and held through depth 57.
- The earlier `MinInterval = 1` model exposed the related day-granular failure
  in 29 steps.

Zero-credit repeats are destructive rather than merely wasted. At stability
6.9 and difficulty 5.0, measured 12 hours apart inside one UTC date:

```text
Again  S 6.9000 -> 0.9701 (-86%)   D 5.00 -> 8.34 (+3.34)   due 7 days -> 1 day
Hard   S 6.9000 -> 6.9000 (+-0)    D 5.00 -> 6.67 (+1.67)   due -> +168h
Easy   S 6.9000 -> 6.9000 (+-0)    D 5.00 -> 3.31 (-1.69)   due -> +216h
```

Recall stability is bit-exact when elapsed days are zero, but difficulty moves
for every rating, and `Again` collapses stability by 86%. The union guard keeps
both that scheduling corruption and same-learn-day replays off the recording
path.

## Reference

- [`backend/internal/usecase/learn.go`](../../../backend/internal/usecase/learn.go) —
  `Clock` and `systemClock`.
- [`backend/internal/domain/learn_day.go`](../../../backend/internal/domain/learn_day.go) —
  `EarnsSchedulingCredit`, `ReviewedWithinLearnDay`, and the learn-day bounds.
- [`backend/internal/usecase/swipe.go`](../../../backend/internal/usecase/swipe.go) —
  the injected clock read and union replay guard.
- [`backend/internal/usecase/swipe_learn_day_test.go`](../../../backend/internal/usecase/swipe_learn_day_test.go) —
  fixed-clock coverage of both replay rules.
