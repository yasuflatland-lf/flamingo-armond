# Inverse `lower*` helper when a shared error classifier serves both promoted and unpromoted callers

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.
> Closely related to [`outcome-union-enforcement.md`](outcome-union-enforcement.md)
> and [`result-union-errors-as-data.md`](result-union-errors-as-data.md).

## Why

When an outcome-union promotion lands in waves — one mutation at a time from the
schema-lint allowlist — a shared error-classifying helper (e.g. `mapAdminRoleError`)
is often consumed by **multiple** callers in the same package, and not every
caller routes validation refusals into outcome data. Two cases are common:

1. **Unpromoted callers**: methods that still return a bare `error` (e.g.
   `adminRoleUsecase.Delete`), where promoting their signature is out of scope
   for the current wave.
2. **Promoted callers without a `Validation` slot**: methods that return a
   typed outcome (e.g. `adminRoleUsecase.Update` returns `UpdateRoleOutcome`)
   but whose outcome variants are domain-specific (`Role`, `SystemRoleConflict`)
   rather than input-validation carriers, so a classifier-emitted
   `InputValidationInfo` has nowhere to land in the outcome.

In both cases the caller needs the helper's classification logic but must end
up with a single `error` return value — either as the bare error or as the
second slot of the outcome tuple. Promoting the shared helper's signature from
`error` to `(*InputValidationInfo, error)` lets callers that **do** route
validation into outcome data consume the tuple directly, but breaks the other
two cases unless every consumer is rewritten in the same wave.

An **inverse `lower*` helper** restores the legacy `error` shape from the new
`(info, err)` tuple. Callers in cases (1) and (2) wrap the classifier's output
via `lower*` and keep their existing signatures unchanged; callers that route
validation into outcome data consume the tuple directly. The shared classifier
stays the single source of truth, and the promotion wave extends only to the
methods that need the new routing.

## What

The shape is small and reversible:

```go
// backend/internal/usecase/admin_role.go

// Promoted callers (e.g. Update) consume mapAdminRoleError's tuple directly.
func mapAdminRoleError(err error, notFoundField, wrap string) (*InputValidationInfo, error) {
    switch {
    case errors.Is(err, repository.ErrRoleNotFound):
        return NewInputValidationInfo(notFoundField, "role not found"), nil
    case errors.Is(err, repository.ErrRoleDuplicate):
        return NewInputValidationInfo("name", "role name already exists"), nil
    case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
        return nil, err
    default:
        return nil, eris.Wrap(err, wrap)
    }
}

// lowerValidationInfo is the inverse for unpromoted callers (e.g. Delete) that
// keep the legacy error-returning signature. Given the (info, err) tuple, it
// returns the propagating error if non-nil, otherwise a *ucerr.ValidationError
// reconstructed from the info, otherwise nil.
func lowerValidationInfo(info *InputValidationInfo, err error) error {
    if err != nil {
        return err
    }
    if info != nil {
        return ucerr.NewValidationError(info.Field, info.Message)
    }
    return nil
}
```

Callers that lack a `Validation` slot (whether unpromoted or promoted-without-Validation)
compose with one extra line:

```go
// adminRoleUsecase.Delete: unpromoted, legacy error-returning signature.
func (u *adminRoleUsecase) Delete(ctx context.Context, id string) error {
    // ...
    if err := u.roles.Delete(ctx, id); err != nil {
        return lowerValidationInfo(mapAdminRoleError(err, "id", "usecase: admin role delete"))
    }
    return nil
}

// adminRoleUsecase.Update: promoted to UpdateRoleOutcome, but the outcome
// has no Validation slot — its variants are Role and SystemRoleConflict.
// The classifier's *InputValidationInfo gets lowered into the error channel.
func (u *adminRoleUsecase) Update(ctx context.Context, id, name string) (UpdateRoleOutcome, error) {
    // ...
    if _, err := u.roles.Update(ctx, id, normalized); err != nil {
        return UpdateRoleOutcome{}, lowerValidationInfo(
            mapAdminRoleError(err, "id", "usecase: admin role update"),
        )
    }
    // ...
}
```

Callers whose outcome carries a `Validation` slot consume the tuple directly:

```go
// adminRoleUsecase.Create: outcome carries Validation, so the carrier
// routes into outcome data.
func (u *adminRoleUsecase) Create(ctx context.Context, name string) (CreateRoleOutcome, error) {
    // ...
    if err != nil {
        info, perr := mapAdminRoleError(err, "name", "usecase: admin role create")
        if perr != nil {
            return CreateRoleOutcome{}, perr
        }
        return CreateRoleOutcome{Validation: info}, nil
    }
    // happy path
}
```

The same pattern appears with `liftValidationErr` (the upstream direction —
`error` → `(*InputValidationInfo, error)`) when a validator function returns a
plain `error` that promoted call sites need to route as data. `liftValidationErr`
and `lowerValidationInfo` are duals — one bridges a legacy `error` into the new
carrier shape, the other bridges the new tuple back to a legacy `error`.

## How to apply

When promoting a single mutation that shares an error-classifying helper with
other callers in the same package:

1. Change the shared helper's signature to `(*InputValidationInfo, error)`.
2. Add a sibling `lower*` helper that re-wraps the tuple into a single `error`
   via `ucerr.NewValidationError` (and respects the propagating `err` if non-nil).
3. Update every call site that lacks a `Validation` outcome slot — unpromoted
   methods AND promoted methods whose outcome variants are domain-specific —
   to wrap the helper's result via `lower*`. No other method-signature changes.
4. Update the call site whose outcome carries a `Validation` slot to consume
   the tuple directly and route the `info` slot into the outcome.

This bounds the promotion's diff to the mutation being promoted plus a single
one-line wrap at each non-routing caller — well inside the 800-line ceiling in
[`.claude/rules/pr-sizing.md`](../../../.claude/rules/pr-sizing.md). Reference:
`backend/internal/usecase/admin_role.go` (`mapAdminRoleError`, `lowerValidationInfo`,
`translateRoleNameErr`) and `backend/internal/usecase/admin_user.go`
(`mapAdminEditMutationError`) paired with `backend/internal/usecase/validators.go`
(`liftValidationErr`) — both pairs show how
promoted outcome methods can share validation classification with error-channel
callers.
