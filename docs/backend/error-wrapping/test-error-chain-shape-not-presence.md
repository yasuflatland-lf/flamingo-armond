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

## Frame-substring assertions: pick a substring unique to one production wrap

Tests that assert "the chain contains a frame whose message includes substring `S`" (e.g. `assertInternalChain(t, err, "usecase: lookup duplicate card after 23505")` in `backend/internal/usecase/helpers_test.go`) silently pass when `S` is too short to discriminate. Two failure modes:

- **Empty substring (`""`).** `strings.Contains(frame, "")` is always `true`, so the assertion becomes "the chain is non-empty" — a strictly weaker check than the surrounding assertion that the error is non-nil.
- **Substring that matches multiple production wraps.** `"usecase: card"` matches every wrap in `card.go`, so a test that intended to pin one call site silently passes for an error originating at a different one. The bug only surfaces when a code rename moves the matched site but the wrong site keeps emitting the substring.

Pick the substring to match exactly one `eris.Wrap` / `eris.Errorf` call in the production package under test. Grep the production source for the proposed substring before committing the test; if it appears in more than one wrap message, lengthen it until it does not.

## Nested wraps inside a single error each need their own pin

When a tx callback (or any nested-call site) wraps each sub-call independently AND the outer caller wraps the whole tx with a different message, a single `assertInternalChain` call on the OUTER wrap silently accepts an inner wrap that has been dropped — the outer frame keeps matching even when the inner sub-op annotation has rotted away.

Concrete: `EditUser` in `backend/internal/usecase/admin_user.go` wraps each sub-call inside its tx callback:

```go
err = u.tx(ctx, func(tx *gorm.DB) error {
    if profilePatch {
        if err := u.users.UpdateTx(...); err != nil {
            if isContextDone(err) { return err }
            return eris.Wrap(err, "usecase: admin user edit: update profile")
        }
    }
    if err := u.userRoles.SetUserRolesTx(...); err != nil {
        if isContextDone(err) { return err }
        return eris.Wrap(err, "usecase: admin user edit: replace roles")
    }
    return nil
})
if err != nil {
    // mapAdminEditMutationError default branch wraps the whole tx with
    // "usecase: admin user edit: tx".
    ...
}
```

The pre-existing `TestAdminUser_EditUser_InfraErrorFromTx` only asserted the outer `"...edit: tx"` frame. A future refactor that dropped the inner `eris.Wrap(err, "...edit: replace roles")` would still pass the test — the outer frame keeps matching — but the log line would lose which sub-op failed (visible only in the `error_chain` JSON, which is what operators triage on).

The fix: assert BOTH frames in the same test (or split into a dedicated test per sub-op):

```go
// backend/internal/usecase/admin_user_test.go
assertInternalChain(t, err, "usecase: admin user edit: tx")
assertInternalChain(t, err, "usecase: admin user edit: replace roles")
```

And a sibling test exercises the OTHER sub-op (`TestAdminUser_EditUser_UpdateTxInfraError_PinsUpdateProfileWrap`) so each per-sub-op wrap has its own pin. Without both pins, only one of the two inner wraps can drift before any test goes red.

The principle generalises: any production wrap that exists primarily to annotate the chain (not to convert sentinel identity, not to add a stack frame at the originating call site) needs its own substring pin. Nested wraps that share an outer frame cannot rely on the outer frame's pin for coverage.
