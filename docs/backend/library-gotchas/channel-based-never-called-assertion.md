# Channel-based "never called" assertion via `select` + `time.After`

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

To assert that a goroutine has NOT been launched (or that a spied method was never called), use a `chan struct{}` that the stub closes when it fires, then `select` on it with a short `time.After` timeout. Fail in the channel branch (the method fired when it should not have); pass in the timeout branch (no call observed).

This is deterministic: any rogue goroutine produces an immediate `t.Fatal`. A `time.Sleep(N)` + length-check alternative silently passes if the goroutine fires after the sleep window.

## Stub skeleton

The stub below is a hypothetical notifier, not a production type.

```go
type stubNotifier struct {
    mu     sync.Mutex
    calls  []notifyCall
    err    error
    called chan struct{} // nil in tests that do not need synchronization
}

func (s *stubNotifier) Notify(_ context.Context, payload string) error {
    s.mu.Lock()
    s.calls = append(s.calls, notifyCall{payload})
    s.mu.Unlock()
    if s.called != nil {
        close(s.called) // safe: each test initializes a fresh channel
    }
    return s.err
}
```

## Negative assertion (must NOT be called)

```go
stub := &stubNotifier{called: make(chan struct{})}

// ... exercise the code under test ...

select {
case <-stub.called:
    t.Fatal("notifier should not be invoked")
case <-time.After(50 * time.Millisecond):
    // ok: no call observed within the window
}
```

## Positive assertion (must be called)

The same `chan struct{}` does double duty for the positive case — block until the goroutine fires or the timeout expires:

```go
stub := &stubNotifier{called: make(chan struct{})}

// ... exercise the code under test ...

select {
case <-stub.called:
    // ok: method was called
case <-time.After(2 * time.Second):
    t.Fatal("expected Notify call within timeout")
}
```

## Close-once safety

The `if s.called != nil { close(s.called) }` guard tolerates stubs constructed without a channel (e.g. a test that never expects a notification and only checks the outcome struct). Initialize the channel only in tests that exercise the goroutine path; leave it `nil` elsewhere.

A channel must never be closed more than once. Because each test constructs a fresh `stubNotifier` with a fresh channel, and `Notify` is called at most once per test scenario, the close-once invariant holds. If the stub needs to count multiple calls, replace the `close` with a `select { case s.called <- struct{}{}: default: }` non-blocking send and read the count after the assertion window.
