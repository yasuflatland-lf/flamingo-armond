# Error classifier helper: pass-through sentinels and typed errors, wrap infra errors with caller-supplied prefix

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.
> Related: [Canonical layer prefix](../../../.claude/rules/error-wrapping.md#canonical-layer-prefix) and
> [Sentinels](../../../.claude/rules/error-wrapping.md#sentinels).

## Why

When multiple usecase methods share a common gate check, each caller needs
infrastructure errors wrapped with its own per-module prefix:

```
"usecase: admin role: check admin"
"usecase: admin user: check admin"
```

A naive approach embeds a fixed `eris.Wrap` inside the helper:

```go
// BAD: helper buries caller attribution in the chain.
func checkAdmin(ctx context.Context, ...) (string, error) {
    // ...
    if err != nil {
        return "", eris.Wrap(err, "usecase: admin gate: check admin") // wrong prefix for every caller
    }
}
```

Every caller now gets `"usecase: admin gate: check admin"` in the log chain
regardless of which file or operation triggered the failure. The caller-specific
module ("admin role", "admin user") is gone; `grep -n 'usecase: admin role:'`
returns nothing for these paths, and `assertInternalChain` cannot pin a stable
per-file substring.

## Pattern: consolidated method with caller-supplied prefix

The canonical solution is a method that accepts the prefix as a parameter,
applies the sentinel pass-through logic internally, and wraps infrastructure
errors with the caller-supplied string. This eliminates the "forgot the
classifier wrap" footgun: the wrap is unconditionally applied inside `Require`,
so there is no separate step the caller can omit.

`AdminGate.Require` in `backend/internal/usecase/admin_gate.go` is the current
implementation:

```go
// AdminGate consolidates admin-authorization for usecase entry points.
type AdminGate struct {
    checker AdminChecker
}

// Require returns the caller's user ID after confirming the bearer is an admin.
// Return paths:
//
//   - ucerr.ErrUnauthenticated — no caller on the context.
//   - context.Canceled / context.DeadlineExceeded — propagated unwrapped.
//   - ucerr.ErrUnauthenticated / *ucerr.ForbiddenError — passed through from IsAdmin.
//   - eris.Wrap(err, callerPrefix) — every other IsAdmin error.
//   - *ucerr.ForbiddenError("admin only") — bearer is not an admin.
//   - nil — bearer is confirmed admin; callerID == caller.Sub.
//
// callerPrefix MUST be non-empty (e.g. "usecase: admin role: check admin").
func (g *AdminGate) Require(ctx context.Context, callerPrefix string) (callerID string, err error) {
    caller := auth.UserFrom(ctx)
    if caller == nil || caller.Sub == "" {
        return "", ucerr.ErrUnauthenticated
    }
    isAdmin, err := g.checker.IsAdmin(ctx, caller.Sub)
    if err != nil {
        if isContextDone(err) || errors.Is(err, ucerr.ErrUnauthenticated) {
            return "", err
        }
        if _, ok := errors.AsType[*ucerr.ForbiddenError](err); ok {
            return "", err
        }
        return "", eris.Wrap(err, callerPrefix)
    }
    if !isAdmin {
        return "", ucerr.NewForbiddenError("admin only")
    }
    return caller.Sub, nil
}
```

Each caller passes its own prefix in one line:

```go
// admin_role.go — (*adminRoleUsecase).List
if _, err := u.adminGate.Require(ctx, "usecase: admin role: check admin"); err != nil {
    return ..., err
}

// admin_user.go — (*adminUserUsecase).List
if _, err := u.adminGate.Require(ctx, "usecase: admin user: check admin"); err != nil {
    return ..., err
}
```

## Design principles

1. **Sentinels and typed errors always pass through.** `ucerr.ErrUnauthenticated`,
   `*ucerr.ForbiddenError`, `context.Canceled`, and `context.DeadlineExceeded`
   must reach the resolver layer unchanged so `gqlerr.FromUsecaseError` can
   classify them correctly. An additional wrap would defeat `errors.Is` /
   `errors.AsType` matching.
2. **Caller owns the prefix.** The prefix encodes the caller's module and
   operation (`"usecase: admin role: check admin"`), not the helper's identity.
   This preserves the two-segment prefix convention and keeps `grep` and
   `assertInternalChain` usable at the per-file level.
3. **The wrap is unconditional for infrastructure errors.** Applying the wrap
   inside `Require` removes the two-step call pattern and ensures no caller can
   accidentally skip the wrap.

## Anti-pattern: helper-internal fixed-prefix wrap causes double-wrapping

If a future refactor embeds a fixed prefix inside the helper and callers wrap
again with their own prefix, the chain carries both messages:

```
"usecase: admin role: check admin": "usecase: admin gate: check admin": <db error>
```

The outer message is correct; the inner one is redundant noise that pollutes the
`error_chain` log attribute and breaks `assertInternalChain` assertions that pin
on a single stable prefix. The fix is always to move the prefix into the caller
parameter rather than hardcoding it in the helper.

## Evolution

This pattern was originally implemented as a split pair: a pure pass-through
helper (`requireAdmin`) that returned infrastructure errors unwrapped, plus a
companion classifier (`wrapAdminGateError`) that callers invoked with their own
prefix string. The split worked correctly but required every caller to remember
the two-step invocation. The pattern was consolidated into `AdminGate.Require`
(issue #215), which accepts `callerPrefix` as a parameter and applies the
classification ladder internally. The underlying invariant — the prefix must
come from the caller, not be hardcoded in the helper — is unchanged; only the
call shape is simpler.
