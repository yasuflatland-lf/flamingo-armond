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
