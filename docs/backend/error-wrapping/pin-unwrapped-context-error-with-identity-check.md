# Pin unwrapped context error identity at the usecase boundary

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.
> Related: [Legacy primitives (resolver-only)](../../../.claude/rules/error-wrapping.md#legacy-primitives-resolver-only),
> [Test the `error_chain` shape, not just its presence](test-error-chain-shape-not-presence.md),
> [Dead context-done branch in pass-through helper](../library-gotchas/dead-context-check-in-pass-through-helper.md).

The usecase contract says `context.Canceled` and `context.DeadlineExceeded`
pass through **unwrapped** so the resolver's `gqlerr.FromUsecaseError` can route
them to `Cancelled(ctx, err)` via `errors.Is`. Tests that only call
`assertCancelled(t, err)` verify the weaker chain-shape contract — they pass
whether the error is the bare sentinel or `eris.Wrap(context.Canceled, "...")`.
A future refactor that adds a wrap on top would not break those tests, but
would silently change the wire-format classification at the resolver.

## Why `assertCancelled` alone is too weak

`assertCancelled` (`backend/internal/usecase/helpers_test.go`) is
`errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)`.
`errors.Is` walks the eris chain, so the assertion succeeds for any error whose
chain contains a context sentinel anywhere — including wrapped variants:

```go
// Both pass assertCancelled, but only the first is contract-compliant
// at the usecase boundary.
err1 := context.Canceled
err2 := eris.Wrap(context.Canceled, "usecase: swipe: find card")
```

The pass-through contract is identity-level: the resolver expects the value it
receives to **be** the context sentinel, not merely to wrap one. A wrap would
still classify correctly today (`gqlerr.FromUsecaseError` uses `errors.Is`),
but the contract documented in the error-wrapping rules is the stronger
identity form, and downstream code that compares via `==` rather than
`errors.Is` (e.g. a future logger that suppresses bare context errors) breaks
silently if the wrap leaks in.

## The dual assertion

Representative tests use `assertCancelled` for the chain check **and**
`require.Equal` for the identity check:

```go
// backend/internal/usecase/swipe_error_test.go
_, err := uc.HandleSwipe(authedCtx("user-1"), HandleSwipeInput{...})

assertCancelled(t, err)
require.Equal(t, context.Canceled, err,
    "expected unwrapped context.Canceled, got %v", err)
```

`assertCancelled` keeps the canonical narrowing-failure message ("expected
context.Canceled or context.DeadlineExceeded in chain"); `require.Equal` adds
the identity pin. The two together fail loudly in both directions: a missing
chain match and a snuck-in wrap.

## Why not strengthen `assertCancelled` itself

`assertCancelled` is a generic helper used across the usecase test suite,
including paths where a wrap is legitimate (e.g. an infrastructure-class error
that happens to carry a cancelled context in its chain). Strengthening the
helper to `require.Equal` would over-constrain those call sites. The
layer-specific strength — usecase-boundary returns are identity-level, deeper
layers are chain-level — belongs at the test level, not the helper level.

## Choosing representative tests

The `require.Equal` pin lives on a small number of structurally distinct
representative tests, not every cancellation test:

| Test | Pinned shape | Why it is representative |
|---|---|---|
| `TestSwipeUsecase_HandleSwipe_FindCardByID_PropagatesCancelled` (`swipe_error_test.go`) | In-tx path | Error originates inside the `txRunner` closure |
| `TestSwipeUsecase_HandleSwipe_ListRecentSwipes_PropagatesDeadlineExceeded` (`swipe_error_test.go`) | Post-tx path | Error originates after the tx commits |
| `TestLearnUsecase_NextDueCards_FindCardgroup_PropagatesCancelled` (`learn_test.go`) | Separate usecase | Confirms the contract holds across usecase boundaries, not just one |

Each test exercises a structurally distinct code path that could regress
independently. Pinning identity on all cancellation tests would be redundant
without strengthening coverage; pinning on these three covers the orthogonal
seams where a wrap could plausibly leak in.

## Self-check

Before merging a cancellation test, complete this sentence: "If a future
refactor wraps the context error with `eris.Wrap`, this test will ___." If
the answer is "still pass", and the test exercises a usecase boundary, add
the `require.Equal` identity pin.
