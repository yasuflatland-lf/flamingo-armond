# `t.Parallel()` + `slog.SetDefault()` mutation is a data race

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

A test that calls `slog.SetDefault()` mutates a package-level global. If that
test also calls `t.Parallel()`, the mutation races against every other parallel
test in the same process that reads the global logger, producing non-deterministic
output and triggering the race detector.

The fix is two-part:

1. **Do not call `t.Parallel()`** on any test that calls `slog.SetDefault()`.
2. **Restore the previous default** in `t.Cleanup` so the mutation is scoped to
   the test even without parallelism.

```go
func TestRecoverFunc(t *testing.T) {
    // Not parallel: mutates the global slog default.
    var buf bytes.Buffer
    prev := slog.Default()
    slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
    t.Cleanup(func() { slog.SetDefault(prev) })

    // ... exercise the code under test ...
}
```

The comment `// Not parallel: mutates the global slog default.` must appear
immediately before the function body so reviewers understand why `t.Parallel()`
is absent. Omitting the comment invites a future editor to "fix" the missing
`t.Parallel()` call and re-introduce the race.

The `t.Cleanup` restore is required even without parallelism: without it, a
subsequent test in the same package that expects the default logger may observe
the handler installed by the previous test. Go's testing package runs subtests
and non-parallel tests in the order they appear in the file, so a missing restore
causes state leakage across test functions.

**Reference:** `backend/internal/gqlerr/errors_test.go` — `TestRecoverFunc`,
`TestInternal_logsError`, and `TestInternal_VariadicAttrs` all follow this pattern.

**Scope after issue #161 landed.** This gotcha applies to handler, auth, and gqlerr tests only — the usecase layer no longer touches `slog.SetDefault`. Every usecase constructor takes an injected `*slog.Logger`; tests pass a discard logger via the `newTestLogger()` helper in `backend/internal/usecase/helpers_test.go` and run unrestricted under `t.Parallel()`.
