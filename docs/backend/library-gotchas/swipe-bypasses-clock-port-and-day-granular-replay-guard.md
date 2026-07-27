# `swipeUsecase` deliberately bypasses `Clock`, and its replay guard is learn-day granular

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

Both deviations below were reviewed on 2026-07-27 and deliberately deferred:
the ambient wall-clock read and the learn-day-granular replay guard are known
constraints, not overlooked defects.

## `swipeUsecase` bypasses the `Clock` port

**The deviation is asymmetric clock ownership.** `LearnUsecase` and
`statsUsecase` hold the shared `usecase.Clock`, accept it in their constructors,
default it to `systemClock`, and read `u.clock.Now().UTC()`. `swipeUsecase`
holds no `Clock`; it reads the ambient wall clock inside its transaction,
immediately before loading the user's FSRS row:

```go
// backend/internal/usecase/swipe.go
now = time.Now().UTC()
byCardID, err := u.userFSRSRepo.FindByUserAndCardIDsTx(
    ctx, tx, user.Sub, []string{card.ID},
)
```

**The main cost is the missing test seam.** The swipe path cannot be exercised
at a chosen learn-day position. A test cannot express "swipe at 23:59:59 JST"
or make one request straddle JST midnight without changing production code.
The learn and stats paths can inject those instants; swipe cannot.

**The existing boundary tests inherit the wall clock.**
`TestSwipeUsecase_HandleSwipe_SameLearnDayRepeat_IsSuccessShapedNoOp` and
`TestSwipeUsecase_HandleSwipe_LearnDayBoundary` in
[`backend/internal/usecase/swipe_learn_day_test.go`](../../../backend/internal/usecase/swipe_learn_day_test.go)
derive their fixtures from `time.Now().UTC()` because they cannot inject a
clock. Structurally, once per day, the 15:00:00 UTC learn-day boundary could
fall between fixture construction and the usecase's own read and invert the
boundary assertions. That race window is only microseconds wide, has never
been observed, and should not be described as an active flake.

**The correctness severity is low today.** The formal skew configuration keeps
`MinInterval = 24`, permits the swipe read to lead the serve read by one hour,
and still holds `NoZeroCreditReview`: 7,122 states were generated, 5,924 were
distinct, and the search depth was 57. See the
[captured learn-day results](../../superpowers/verification/learn-session-invariants/README.md#learn-day-results).
That evidence classifies this as a testability and consistency problem under
current scheduling, not a demonstrated production correctness failure.

**The risk becomes reachable if time must be shared across operations.** A
future rule that depends on a reproducible request/session instant, or that
requires serve and swipe to agree across independently clocked instances,
would turn the ambient read into a correctness input that tests cannot control.

**The deferred fix is ordinary constructor injection.** Add `clock Clock` to
`swipeUsecase`, `NewSwipeUsecase`, and `NewSwipeUsecaseWithTx`; default nil to
`systemClock{}`; replace the ambient read with `u.clock.Now().UTC()`; and update
the learn-day tests to use a fixed clock. That is intentionally not part of the
current change.

## The replay guard is learn-day granular

**The deviation is between calendar-day and elapsed-time predicates.**
The recording path treats any repeat within the same JST learn day as a replay:

```go
// backend/internal/usecase/swipe.go
if existing != nil &&
    domain.ReviewedWithinLearnDay(existing.State.LastReview, now) {
    return nil
}
```

The underlying FSRS rule in
[`backend/internal/domain/learn_day.go`](../../../backend/internal/domain/learn_day.go)
is elapsed-time based. FSRS floors hours divided by 24 to derive elapsed days,
so a repeat within 24 hours has retrievability 1 and a stability growth factor
of exactly zero. Calendar-day membership is not the same predicate: a card
reviewed at 23:00 JST and recorded again after the 00:00 rollover is only one
hour old, but the day-granular guard no longer fires.

**The counterexample is unreachable in production today.**
`service.NewFSRSScheduler` sets `params.EnableShortTerm = false` in
[`backend/internal/domain/service/fsrs_scheduler.go`](../../../backend/internal/domain/service/fsrs_scheduler.go),
so current scheduling emits whole-day intervals. The formal long-term
configuration holds `NoZeroCreditReview` with 6,278 generated states, 5,485
distinct states, and depth 57. Production configuration therefore cannot
re-serve the one-hour-old card needed to exercise the recording-side gap.

**Short-term scheduling makes it reachable.** If `EnableShortTerm` is turned
back on, or another scheduler begins emitting intervals shorter than 24 hours,
a card can become due after the JST date changes but before a whole elapsed day.
[`backend/internal/domain/mastery_tier.go`](../../../backend/internal/domain/mastery_tier.go)
already records that the flag may be revisited.

**The model shows the exact failure.** With `MinInterval = 1`,
`NoZeroCreditReview` fails at State 29. A card is reviewed at hour 23, becomes
due at hour 24, crosses the learn-day boundary, passes the day-granular
recording guard, and is reviewed again with `credits=<<24, 1>>`. See the
[captured counterexample](../../superpowers/verification/learn-session-invariants/README.md#learn-day-results).

**The deferred fix belongs on the recording side.** If sub-day intervals return,
replace `ReviewedWithinLearnDay` in the swipe replay check with a domain-owned
elapsed-time predicate that treats `now.Sub(lastReview) < 24*time.Hour` as a
replay, with exact-boundary and cross-midnight tests. Keep that predicate in the
domain rather than spelling duration arithmetic directly in the usecase.

## Reference

- [`backend/internal/usecase/learn.go`](../../../backend/internal/usecase/learn.go) —
  `Clock`, `systemClock`, and the learn-path reads.
- [`backend/internal/usecase/stats.go`](../../../backend/internal/usecase/stats.go) —
  the stats-path `Clock` injection.
- [`backend/internal/usecase/swipe.go`](../../../backend/internal/usecase/swipe.go) —
  the ambient read and recording-side replay guard.
- [`docs/superpowers/verification/learn-session-invariants/README.md`](../../superpowers/verification/learn-session-invariants/README.md#learn-session-invariant-models) —
  executable models, captured output, and rerun commands.
