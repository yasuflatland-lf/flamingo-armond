# `liftValidationErr` shadow-assignment pattern and dead-code trap at promoted call sites

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.
> Closely related to [`outcome-union-enforcement.md`](outcome-union-enforcement.md),
> [`input-validation-info-empty-field-panic.md`](input-validation-info-empty-field-panic.md),
> and [`inverse-helper-for-partial-promotion.md`](inverse-helper-for-partial-promotion.md).

## Why

`liftValidationErr` (declared in `backend/internal/usecase/admin_user.go`) is the
shared helper that bridges a validator returning `error` into the outcome-union
shape: it unwraps a `*ucerr.ValidationError` into an `*InputValidationInfo`
carrier and passes every other error through unchanged. Its return contract is:

```go
func liftValidationErr(err error) (*InputValidationInfo, error) {
    if err == nil {
        return nil, nil
    }
    if ve, ok := errors.AsType[*ucerr.ValidationError](err); ok {
        return NewInputValidationInfo(ve.Field, ve.Message), nil
    }
    return nil, err
}
```

Two consequences of the contract matter at every promoted call site, and both
are easy to get wrong:

1. **`(nil, nil)` is reachable only on a nil `err` input.** Inside an
   `if err != nil { ... }` block — which is the canonical placement for the
   helper call — the helper can never return `(nil, nil)`. Either the input
   was a `*ucerr.ValidationError` and the first slot is non-nil, or it was
   some other error and the second slot is non-nil. A trailing
   `return XOutcome{}, err` after both `if info != nil` and `if err != nil`
   branches is dead code; static analysis will not flag it because the
   helper's signature allows it in principle, but the local control flow
   has already exhausted every reachable case.

2. **Shadow the outer `err` with `:=`, do not rename.** The canonical idiom
   across `backend/internal/usecase/` is `info, err := liftValidationErr(err)`
   — the inner `err` shadows the outer one so the subsequent `if err != nil`
   branches on the helper's residual. Introducing a custom name
   (`lifted`, `validationErr`, `bridged`) makes the call site read
   inconsistently with the precedent set by `cardgroup.go`, `card.go`,
   `user.go`, `admin_user.go`, and `swipe.go`, and it leaves the original
   outer `err` live in scope — a subsequent edit that mistakenly references
   it gets the pre-lift value, not the residual.

## What

The correct shape, from `backend/internal/usecase/swipe.go` (the
`authorizeCardgroup` branch and the post-tx-closure branch share the same
local pattern):

```go
if err := u.authorizeCardgroup(ctx, in.CardgroupID, user.Sub); err != nil {
    // authorizeCardgroup returns ucerr.NewValidationError("cardgroupId", ...) for
    // not-found and ucerr.ErrUnauthenticated for non-owner. The not-found case
    // is a validation variant; the non-owner case stays on the error channel.
    info, err := liftValidationErr(err)
    if err != nil {
        return HandleSwipeOutcome{}, err
    }
    if info != nil {
        return HandleSwipeOutcome{Validation: info}, nil
    }
    // No trailing `return XOutcome{}, err` here — both branches above are
    // exhaustive when liftValidationErr is called with a non-nil input.
}
```

The same two-branch shape appears in `card.go` (`Update`), `cardgroup.go`
(`Create`, `Update`), `user.go` (`UpdateUser`), and `admin_user.go`
(`UpdateUser`) — count grep:

```bash
grep -n 'liftValidationErr' backend/internal/usecase/*.go | grep -v _test.go
```

## How to apply

When promoting a usecase method that calls a validator returning `error`:

- Place the `info, err := liftValidationErr(...)` line inside the `if err != nil`
  block that already exists for the validator's return — never at the
  top level of the method.
- After `if err != nil { return ..., err }` and `if info != nil { return ..., nil }`,
  do not add a final `return XOutcome{}, err` — it is structurally unreachable.
- Use the `info, err :=` shadow form, not a renamed second variable. The
  shared name keeps grep, lint, and future-promoter eyes on the canonical
  shape.
- If a method needs both a `liftValidationErr` step and a follow-up
  validator that also returns `error`, run them in sequence and overwrite
  `info` and `err` on each step — see `backend/internal/usecase/user.go`
  for the two-step (`displayName` then `bio`) pattern.
