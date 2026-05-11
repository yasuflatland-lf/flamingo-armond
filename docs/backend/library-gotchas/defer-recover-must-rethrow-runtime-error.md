# `defer recover()` must re-panic `runtime.Error`

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

A blanket `recover()` in a `defer` collapses two unrelated failure modes into one user-facing parse error: a real bug like a nil-deref or index-out-of-range (`runtime.Error`) and a legitimate "we asked the parser to give up on this input" panic raised by hand. The runtime.Error case is a server bug and must surface in tests, CI, and crash reports — not get rewritten as `ValidationError`. The textdic parser uses this guard:

```go
defer func() {
    if r := recover(); r != nil {
        if rt, ok := r.(runtime.Error); ok {
            panic(rt)
        }
        err = fmt.Errorf("textdic: parser panic: %v\n%s", r, debug.Stack())
    }
}()
```

Apply the same shape to any new `recover` site that wraps third-party generated code (goyacc parsers, regex engines, text-processing libraries). The `debug.Stack()` capture is mandatory: by the time the recovered error is logged, the original goroutine stack is gone.

### Cross-reference: gqlgen `srv.SetRecoverFunc`

gqlgen calls the registered `RecoverFunc` from inside its own `defer`, so the goroutine
stack at the recovery point is the panic site (not yet unwound). `debug.Stack()` captured
there therefore preserves the panic origin. Wrap the recovered value via `eris.Errorf` and
pass it to `gqlerr.Internal(ctx, ...)` — the wire response remains the fixed
`"internal server error"` message regardless of the wrapped error content. See
[`panic-value-format-pii-tradeoff.md`](panic-value-format-pii-tradeoff.md) for the
`%T %v` vs `%T`-only format decision and the associated log-PII trade-off.
