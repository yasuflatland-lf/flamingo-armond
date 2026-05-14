# `t.Setenv` vs `os.Setenv` in tests

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

`t.Setenv(key, value)` sets an environment variable for the duration of the test
and automatically restores the original value (or unsets it if it did not exist)
when the test ends. This makes it safe for table-driven tests — each subtest gets
its own scoped mutation — without manual `defer os.Unsetenv(key)` cleanup.

`os.Setenv` does **not** restore the original value. Using it in a test leaks the
mutation to every subsequent test in the same process, producing order-dependent
failures and intermittent CI flakiness.

```go
// WRONG — mutation leaks to subsequent tests
func TestFoo(t *testing.T) {
    os.Setenv("SHUTDOWN_TIMEOUT", "5s")
    defer os.Unsetenv("SHUTDOWN_TIMEOUT") // easy to forget; also races with t.Parallel subtests
    // ...
}

// CORRECT — automatically restored after each subtest
func TestServerConfigFromEnv(t *testing.T) {
    cases := []struct {
        name string
        env  string
        want time.Duration
    }{
        {"empty uses default", "", defaultShutdownTimeout},
        {"valid positive", "5s", 5 * time.Second},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            t.Setenv("SHUTDOWN_TIMEOUT", tc.env)
            cfg := serverConfigFromEnv(logger)
            // assert on cfg...
        })
    }
}
```

**Note:** `t.Setenv` calls `t.Helper()` internally and calls `t.Fatal` if the test
(or any parent) is marked parallel with `t.Parallel()`, because env-var mutations
are not safe to run concurrently. If a test needs both parallelism and env-var
isolation, consider using a fake environment (e.g. pass a `map[string]string`
through a dependency-injected lookup function) rather than the process-level env.

**Reference:** `backend/cmd/server/main_test.go` — `TestServerConfigFromEnv`
uses `t.Setenv` for every env-var case in the table.
