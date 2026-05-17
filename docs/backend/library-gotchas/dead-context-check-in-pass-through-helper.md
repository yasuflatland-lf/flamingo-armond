# Dead context-done branch in pass-through helper — collapse to single return

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

A helper that returns errors unchanged ("raw pass-through") must not also
inspect the error for context cancellation internally. The two responsibilities
contradict each other: a pass-through helper's contract is to return whatever
it received; adding an internal context-done branch implies it will do something
different for that branch — but it doesn't.

## The antipattern

```go
// AVOID: the isContextDone branch is dead code.
// Both branches return the same `err`; the reader is misled into thinking
// the caller will be notified differently for context errors.
func resolveCallerRole(ctx context.Context, svc RoleService, sub string) (string, error) {
    isAdmin, err := svc.IsAdmin(ctx, sub)
    if err != nil {
        if isContextDone(err) {
            return "", err // <-- same as the branch below; context check adds nothing
        }
        return "", err
    }
    // ...
}
```

The `isContextDone` check is dead code: both arms of the `if` return `("", err)`
unchanged. A reader who sees the check infers "when context is cancelled, the
caller handles it specially" — but no such handling exists. The mismatch costs
attention and trust.

## The fix

```go
// PREFER: state the contract in one line.
// The caller is responsible for all error classification.
func resolveCallerRole(ctx context.Context, svc RoleService, sub string) (string, error) {
    isAdmin, err := svc.IsAdmin(ctx, sub)
    if err != nil {
        return "", err
    }
    // ...
}
```

## When is context inspection appropriate inside a helper?

Context inspection is only warranted when the helper performs **different
behaviour** based on the error type:

```go
// Legitimate: helper wraps infrastructure errors but lets context errors
// escape unwrapped so the caller's errors.Is(err, context.Canceled) works.
func fetchWithRetry(ctx context.Context, fn func() error) error {
    for {
        err := fn()
        if err == nil {
            return nil
        }
        if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
            return err // pass through unwrapped
        }
        // wrap and potentially retry infrastructure errors
        return eris.Wrap(err, "fetchWithRetry: infrastructure")
    }
}
```

Here the branching changes the returned value — unwrapped vs. wrapped — so the
check is load-bearing.

## Self-check

Before adding a context-done check inside a helper, complete this sentence:
"If the error is a context error, this helper will ___; otherwise it will ___."
If the two blanks are identical, the check is dead code — remove it.
