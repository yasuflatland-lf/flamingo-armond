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

## Per-sub-op wrap inside a tx callback must skip context errors

When a tx callback wraps each sub-call with its own operation prefix to
preserve `error_chain` attribution (see [Nested wraps inside a single error
each need their own pin](test-error-chain-shape-not-presence.md#nested-wraps-inside-a-single-error-each-need-their-own-pin)),
the wrap MUST be skipped for `context.Canceled` / `context.DeadlineExceeded`
so the identity contract above survives across the tx layer. The canonical
shape is:

```go
err = u.tx(ctx, func(tx *gorm.DB) error {
    if err := u.users.UpdateTx(ctx, tx, id, patch); err != nil {
        if isContextDone(err) {
            return err  // bare identity preserved through tx
        }
        return eris.Wrap(err, "usecase: admin user edit: update profile")
    }
    if err := u.userRoles.SetUserRolesTx(ctx, tx, id, roleIDs); err != nil {
        if isContextDone(err) {
            return err
        }
        return eris.Wrap(err, "usecase: admin user edit: replace roles")
    }
    return nil
})
if err != nil {
    if isContextDone(err) {
        return AdminEditUserOutcome{}, err  // bare identity returned to caller
    }
    // ... wrap or classify ...
}
```

Two independent guarantees combine here:

- **Identity preservation across the tx layer.** Without the inner
  `isContextDone` check, a cancellation that fires inside `UpdateTx` would
  surface to the outer `if err != nil` branch as
  `eris.Wrap(context.Canceled, "...update profile")`. `errors.Is` still
  detects the cancellation, but the value returned to the resolver is no
  longer the bare sentinel — breaking any downstream consumer that compares
  via `==`. The inner short-circuit keeps the wrap convention applied only to
  infra-class errors.
- **Outer fallback wrap also skips ctx-done.** The post-tx classification
  branch (`mapAdminEditMutationError` in the worked example) likewise checks
  `isContextDone` before its `eris.Wrap` default — without this the outer
  fallback would re-wrap the bare sentinel.

Pin the contract with two tests per sub-op:

| Test | Asserts |
|---|---|
| `TestAdminUser_EditUser_UpdateTxCancelled` | `context.Canceled` identity (`err == context.Canceled`) on the profile branch |
| `TestAdminUser_EditUser_CancelledFromRoleSet` | Same identity on the roles branch |

A single cancellation test that only checks chain shape (`assertCancelled`)
passes even when the inner wrap snuck in. The dual-assertion pattern from
[The dual assertion](#the-dual-assertion) above applies inside the tx-callback
too — each sub-op branch that can carry a context error needs its own
identity pin, not a chain-shape pin shared with the others.

## Multi-call free-function helpers: guard every branch, not just the first

The same per-branch discipline applies outside a tx callback. A free-function
helper that makes two or more sequential infrastructure calls must place
`if isContextDone(err) { return nil, err }` on **each** call's error branch
before the `eris.Wrap`. Guarding only the first call while wrapping the second
silently double-wraps a `context.Canceled` / `context.DeadlineExceeded` from
the second call, breaking the bare-identity contract for that path even though
the chain still satisfies `errors.Is`.

Worked example: `checkCardgroupLimit` (`backend/internal/usecase/cardgroup.go`)
calls `admin.IsAdmin` then `counter.CountByOwner`. Both error branches guard
context errors via `isContextDone`. An asymmetric version that guarded only the
`IsAdmin` branch shipped briefly and was caught in review — the
`CountByOwner`-cancelled path returned a wrapped error that failed an
`err == context.Canceled` identity check at the caller. The fix mirrored the
guard onto the second branch.

Pin the contract with identity tests on each branch:
`TestCheckCardgroupLimit_CountCancelled_IdentityPreserved` and
`_CountDeadlineExceeded_IdentityPreserved` assert `err == context.Canceled` /
`context.DeadlineExceeded` (bare identity) for the count path, complementing
the `IsAdmin`-branch tests.
