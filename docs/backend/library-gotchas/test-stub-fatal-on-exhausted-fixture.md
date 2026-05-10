# Test stubs `t.Fatalf` on exhausted fixture access, never panic

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

A queue-style test stub that pops responses from a slice (`resp := s.responses[id][0]; s.responses[id] = s.responses[id][1:]`) crashes with `panic: runtime error: index out of range` if the test calls the stub more times than fixtures were provided. The panic stack trace points at the stub, not at the test that miscounted, and the failing test name is buried under the panic header.

**Fix:** give the stub a `*testing.T` field at construction and bounds-check before the pop:

```go
type stubBlockService struct {
    t         *testing.T
    responses map[string][]Response
}

func (s *stubBlockService) GetChildren(ctx context.Context, id string) (Response, error) {
    if len(s.responses[id]) == 0 {
        s.t.Fatalf("stub: unexpected GetChildren call for id=%q (responses exhausted)", id)
    }
    resp := s.responses[id][0]
    s.responses[id] = s.responses[id][1:]
    return resp, nil
}
```

**Why:** `t.Fatalf` produces a normal test failure with the offending key in the message — the failing test name is preserved and CI output points directly at the over-invocation. Compare to `panic` which surfaces as an opaque stack trace that takes a second pass to attribute.

**How to apply:** every test stub whose lifetime is bounded by a `*testing.T` should hold a reference to it. The cost is one extra struct field; the payoff is intelligible failures when fixture counts and call counts drift.

**Reference:** `stubBlockService` in `backend/internal/notion/fetcher_test.go` adopted this pattern after a fixture-count mismatch was about to surface as a stack trace.
