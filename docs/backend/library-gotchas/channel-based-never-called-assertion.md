# Channel-based "never called" assertion via `select` + `time.After`

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

To assert that a goroutine has NOT been launched (or that a spied method was never called), use a `chan struct{}` that the stub closes when it fires, then `select` on it with a short `time.After` timeout. Fail in the channel branch (the method fired when it should not have); pass in the timeout branch (no call observed).

This is deterministic: any rogue goroutine produces an immediate `t.Fatal`. A `time.Sleep(N)` + length-check alternative silently passes if the goroutine fires after the sleep window.

## Stub skeleton

```go
type stubNotionWriter struct {
    mu     sync.Mutex
    calls  []writeCall
    err    error
    called chan struct{} // nil in tests that do not need synchronization
}

func (s *stubNotionWriter) AppendParagraph(_ context.Context, pageID, text string) error {
    s.mu.Lock()
    s.calls = append(s.calls, writeCall{pageID, text})
    s.mu.Unlock()
    if s.called != nil {
        close(s.called) // safe: each test initializes a fresh channel
    }
    return s.err
}
```

## Negative assertion (must NOT be called)

```go
stub := &stubNotionWriter{called: make(chan struct{})}

// ... exercise the code under test ...

select {
case <-stub.called:
    t.Fatal("write-back should not be invoked")
case <-time.After(50 * time.Millisecond):
    // ok: no call observed within the window
}
```

## Positive assertion (must be called)

The same `chan struct{}` does double duty for the positive case — block until the goroutine fires or the timeout expires:

```go
stub := &stubNotionWriter{called: make(chan struct{})}

// ... exercise the code under test ...

select {
case <-stub.called:
    // ok: method was called
case <-time.After(2 * time.Second):
    t.Fatal("expected AppendParagraph call within timeout")
}
```

## Close-once safety

The `if s.called != nil { close(s.called) }` guard tolerates stubs constructed without a channel (e.g. a no-writeback test that only checks the outcome struct). Initialize the channel only in tests that exercise the goroutine path; leave it `nil` elsewhere.

A channel must never be closed more than once. Because each test constructs a fresh `stubNotionWriter` with a fresh channel, and `AppendParagraph` is called at most once per test scenario, the close-once invariant holds. If the stub needs to count multiple calls, replace the `close` with a `select { case s.called <- struct{}{}: default: }` non-blocking send and read the count after the assertion window.
