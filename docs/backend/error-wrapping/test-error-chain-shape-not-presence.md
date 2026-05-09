# Test the `error_chain` shape, not just its presence

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.

A test that asserts only `rec0["error_chain"] != nil` passes whether the value is the rich `{root: {stack: [...]}, wrap: [...]}` shape or the degraded `{external: "..."}` shape that `eris.ToJSON` emits for stdlib errors. The degraded shape is exactly what slips in when a stub returns `errors.New(...)` — the test goes green, the log line ships with no stack, and operators triaging the WARN have nothing to grep for. Assert the structural shape:

```go
chain, ok := rec0["error_chain"].(map[string]any)
if !ok { t.Fatalf("error_chain is not a JSON object: %T", rec0["error_chain"]) }
root, hasRoot := chain["root"].(map[string]any)
if !hasRoot {
    t.Error("error_chain must have root entry (got external-only shape; stub may be using stdlib errors)")
}
if root != nil {
    if stack, _ := root["stack"].([]any); len(stack) == 0 {
        t.Error("error_chain.root.stack must contain at least one frame")
    }
}
```

Test stubs that produce errors must also use `eris.New(...)` (not `errors.New(...)`) so the assertion exercises the same code path production hits. Pattern in `backend/internal/auth/superuser_test.go` (`M7_IsAdminError`, `M8_AssignToUserError`).

**A single `eris.New` stub is not enough to prove production's `eris.Wrap` is load-bearing.** A test where the stub always returns an eris-wrapped error passes the `error_chain.root.stack` assertion whether or not production wraps the error — because the stub's own eris chain provides the root. Add a sibling test that stubs the *actual* stdlib sentinel (e.g. `repository.ErrNotFound`, a plain `errors.New` value) and still asserts the rich shape. Only that test will fail if someone removes the production `eris.Wrap` call at the log site. Reference: `backend/internal/usecase/card_test.go` (`TestCardUsecase_Create_DuplicateLookupRace_RowVanished`) — the documented race where the duplicate row vanishes before the follow-up SELECT returns `repository.ErrNotFound`, and the test proves that the usecase's `eris.Wrap(lookupErr, ...)` is what makes the chain rich.
