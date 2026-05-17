# `NewInputValidationInfo` panics on empty `Field` — mirror the `ucerr.NewValidationError` invariant

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.
> Closely related to [`result-union-errors-as-data.md`](result-union-errors-as-data.md) and
> [`outcome-union-enforcement.md`](outcome-union-enforcement.md).

## Why

`InputValidationInfo` (declared in `backend/internal/usecase/admin_user.go`) is the
usecase-layer carrier for an input-validation failure routed as **data** through an
outcome union, rather than as an error. When the resolver maps the carrier to a
`model.InputValidationError` variant, the `Field` value surfaces verbatim on the
GraphQL wire as `extensions.field`. An empty `Field` produces `extensions.field == ""`,
which the frontend cannot bind to any input element — the field-level banner has
no target and either renders against the wrong input or silently disappears.

The wire-side constructor `ucerr.NewValidationError(field, message)` already panics
on an empty `field` for exactly this reason (see [`.claude/rules/error-wrapping.md` §
"Sentinels"](../../../.claude/rules/error-wrapping.md#sentinels) and the constructor's
docblock). The data-side carrier MUST mirror the invariant; otherwise a producer
that accidentally constructs `&InputValidationInfo{Field: "", Message: "..."}` via
the struct literal slips past every compile-time and lint check and the UX rot
surfaces only when a user hits the validation path in production.

## What

Always construct `InputValidationInfo` via `NewInputValidationInfo(field, message)`,
never via the bare struct literal. The constructor panics on empty `field`:

```go
// backend/internal/usecase/admin_user.go
func NewInputValidationInfo(field, message string) *InputValidationInfo {
    if field == "" {
        panic("usecase.NewInputValidationInfo: field must be non-empty")
    }
    return &InputValidationInfo{Field: field, Message: message}
}
```

```go
// Correct
return AssignRoleOutcome{
    Validation: usecase.NewInputValidationInfo("userId", "user not found"),
}, nil

// Forbidden — empty field silently reaches the frontend and the banner has
// no input to bind to.
return AssignRoleOutcome{
    Validation: &usecase.InputValidationInfo{Field: "", Message: "..."},
}, nil
```

The same compile-time guarantee that protects `ucerr.NewValidationError` against a
silent missing-pointer fallthrough applies here: `Error()` is defined on the pointer
receiver of `ucerr.ValidationError`, and a bare `ucerr.ValidationError{...}` value
fails to satisfy the `error` interface. `InputValidationInfo` is not an `error` type
so that compile guarantee does not extend automatically; the constructor-panic
invariant is the only line of defence and must be the single construction path in
production code.

## How to apply

- Any new outcome variant that carries an `*InputValidationInfo` slot follows the
  same construction rule — use `NewInputValidationInfo(field, message)` in the
  usecase, never the struct literal.
- Test code is explicitly exempt by the same rationale as the `ucerr.ValidationError`
  exemption in [`.claude/rules/error-wrapping.md`](../../../.claude/rules/error-wrapping.md):
  tests legitimately construct invalid shapes to verify classifier coverage.
- Reference: `backend/internal/usecase/admin_user.go` (`NewInputValidationInfo`,
  `liftValidationErr`, `mapRoleAssignmentError`) and `backend/internal/usecase/admin_role.go`
  (`mapAdminRoleError`, `normalizeAndValidateRoleName`) — all production
  construction sites flow through the constructor.
