# Error classifier helper: pass-through sentinels and typed errors, wrap infra errors with caller-supplied prefix

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.
> Related: [Canonical layer prefix](../../../.claude/rules/error-wrapping.md#canonical-layer-prefix) and
> [Sentinels](../../../.claude/rules/error-wrapping.md#sentinels).

## Why

When multiple usecase methods share a common gate check (e.g. `requireAdmin`),
each caller needs the returned error wrapped with its own per-module prefix:

```
"usecase: admin role: check admin"
"usecase: admin user: check admin"
"usecase: dictionary: upsert"
```

A naive approach embeds `eris.Wrap` inside the helper:

```go
// BAD: helper buries caller attribution in the chain.
func requireAdmin(ctx context.Context, svc AdminChecker) (string, error) {
    // ...
    if err != nil {
        return "", eris.Wrap(err, "usecase: admin gate: check admin") // wrong prefix for every caller
    }
}
```

Every caller now gets `"usecase: admin gate: check admin"` in the log chain
regardless of which file or operation triggered the failure. The caller-specific
module ("admin role", "admin user", "dictionary") is gone; `grep -n 'usecase: admin role:'`
returns nothing for these paths, and `assertInternalChain` cannot pin a stable
per-file substring.

When N callers each duplicate the classification ladder inline the problem
compounds: the same `isContextDone / errors.Is / errors.As / eris.Wrap` block
appears 11 times and diverges over time.

## Pattern

Extract a pure classifier helper that accepts the error and the caller-supplied
prefix. Sentinels and typed errors pass through unchanged; raw infrastructure
errors are wrapped with the provided prefix.

```go
// backend/internal/usecase/admin_gate.go

// requireAdmin returns the caller's user ID after confirming admin status.
// Sentinels (ErrUnauthenticated, ForbiddenError) and context errors pass
// through; infrastructure errors are returned unwrapped. Callers must wrap
// the returned error via wrapAdminGateError.
func requireAdmin(ctx context.Context, svc AdminChecker) (callerID string, err error) {
    caller := auth.UserFrom(ctx)
    if caller == nil || caller.Sub == "" {
        return "", ucerr.ErrUnauthenticated
    }
    if svc == nil {
        return "", eris.New("usecase: admin gate: admin checker not configured")
    }
    isAdmin, err := svc.IsAdmin(ctx, caller.Sub)
    if err != nil {
        return "", err // pass-through — callers apply their prefix via wrapAdminGateError
    }
    if !isAdmin {
        return "", ucerr.NewForbiddenError("admin only")
    }
    return caller.Sub, nil
}

// wrapAdminGateError classifies the error from requireAdmin.
// Sentinels and context errors pass through; unknown infrastructure errors
// are wrapped with callerPrefix (e.g. "usecase: admin role: check admin").
func wrapAdminGateError(err error, callerPrefix string) error {
    if err == nil {
        return nil
    }
    if isContextDone(err) || errors.Is(err, ucerr.ErrUnauthenticated) {
        return err
    }
    if _, ok := errors.AsType[*ucerr.ForbiddenError](err); ok {
        return err
    }
    return eris.Wrap(err, callerPrefix)
}
```

Each caller applies its own prefix in one line:

```go
// admin_role.go
if _, err := requireAdmin(ctx, u.auth); err != nil {
    return ..., wrapAdminGateError(err, "usecase: admin role: check admin")
}

// admin_user.go
if _, err := requireAdmin(ctx, u.auth); err != nil {
    return ..., wrapAdminGateError(err, "usecase: admin user: check admin")
}

// dictionary.go
if _, err := requireAdmin(ctx, u.auth); err != nil {
    return ..., wrapAdminGateError(err, "usecase: dictionary: upsert")
}
```

## Design principles

1. **Helper is classification-only.** `wrapAdminGateError` decides whether an
   error passes through or gets wrapped; it never decides the prefix string.
2. **Sentinels and typed errors always pass through.** `ucerr.ErrUnauthenticated`,
   `*ucerr.ForbiddenError`, `context.Canceled`, and `context.DeadlineExceeded`
   must reach the resolver layer unchanged so `gqlerr.FromUsecaseError` can
   classify them correctly.
3. **Caller owns the prefix.** The prefix encodes the caller's module and
   operation (`"usecase: admin role: check admin"`), not the helper's identity.
   This preserves the two-segment prefix convention and keeps `grep` and
   `assertInternalChain` usable at the per-file level.
4. **One helper, N callers.** A single `wrapAdminGateError` function consolidates
   the classification ladder that would otherwise be duplicated at every call site.

## Anti-pattern: helper-internal wrap causes double-wrapping

If the helper wraps with its own prefix and callers wrap again with their prefix,
the chain carries both messages:

```
"usecase: admin role: check admin": "usecase: admin gate: check admin": <db error>
```

The outer message is correct; the inner one is redundant noise that pollutes the
`error_chain` log attribute and breaks `assertInternalChain` assertions that pin
on a single stable prefix. Removing the helper-internal wrap is the fix.

## Evolution in this codebase

`admin_gate.go` went through three iterations (commits 91b10cd → 995276d → da42b0a):

- **91b10cd**: helper wrapped internally; `admin_user.go` and `dictionary.go`
  re-wrapped on top — double-wrap for those two callers, inconsistent for the rest.
- **995276d**: all 5 callers re-wrap to achieve consistency — double-wrap now
  universal but still wrong.
- **da42b0a**: helper made pass-through; `wrapAdminGateError(err, callerPrefix)`
  introduced; 11 callers each reduced to one line with their own prefix.
