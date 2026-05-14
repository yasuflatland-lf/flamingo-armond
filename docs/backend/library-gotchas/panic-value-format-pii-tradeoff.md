# Panic value format: `%T %v` vs `%T`-only — PII trade-off

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

When a panic is recovered via `srv.SetRecoverFunc`, the panic value (`any`) may carry
user-influenced content — for example, a string panicked deep inside a parsing library
that was processing user input. Two format choices exist when wrapping that value:

| Choice | Log content | Wire response |
|---|---|---|
| `%T` only | Type name (e.g. `*runtime.TypeAssertionError`) | Fixed `"internal server error"` |
| `%T %v` | Type name + panic message (e.g. `runtime.plainError panic here`) | Fixed `"internal server error"` |

## Why `%v` is safe wire-side

`gqlerr.Internal` fixes the client-visible message regardless of what error is wrapped:

```go
// backend/internal/gqlerr/recover.go  (wired via srv.SetRecoverFunc(gqlerr.RecoverFunc))
func RecoverFunc(ctx context.Context, err any) error {
    stack := debug.Stack()
    return gqlerr.Internal(ctx,
        eris.Errorf("graphql: panic recovered (%T %v)\n%s", err, err, stack),
    )
}
```

The `%v` content is embedded inside the eris chain, which lands exclusively in the
structured ERROR log (via `gqlerr.Internal` → `logging.LogError`). The JSON envelope
sent to the client always reads `{"message": "internal server error", "extensions": {"code": "INTERNAL"}}`.
User-supplied text never reaches the wire through this path.

## The trade-off

Choose `%T %v` when triage value outweighs log-PII surface area in your threat model:

- **Benefit**: the panic message is preserved in the `error_chain` field of the ERROR log,
  making root-cause triage faster — especially for panics in third-party libraries where
  the message is the only actionable signal before the stack trace.
- **Cost**: if the panic value contains user content (e.g. a string derived from request
  input), that string lands in the structured log. Logs are a controlled operator surface,
  but they are not redacted by the wire-side mechanism.

If your threat model requires log redaction for user content, pair `%T %v` with a SIEM-layer
redaction rule rather than dropping the panic message entirely. The `debug.Stack()` capture
is always mandatory — see [`defer-recover-must-rethrow-runtime-error.md`](defer-recover-must-rethrow-runtime-error.md)
for why the stack must be captured at the recovery point.

## What `%T`-only gives up

`panic("some message")` with `%T`-only logs `runtime.plainError` (or the concrete type),
but not the message text. For panics inside generated code or unfamiliar libraries, losing
the message often means the stack trace is the only clue — and the stack may not contain
enough context if the panic site is inside an opaque runtime path.
