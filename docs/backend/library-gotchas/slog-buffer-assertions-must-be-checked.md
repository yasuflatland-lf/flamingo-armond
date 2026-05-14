# Log output assertions: always read the buffer or the assertion is dead code

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

Tests of functions that call `slog.Default()` (or accept a `*slog.Logger`)
commonly redirect log output to a `bytes.Buffer` to avoid polluting test output
and to assert on what was logged. A buffer that is wired up but never read gives
false confidence: the test passes whether the function logs the expected line or
nothing at all.

```go
// WRONG — buf is never read; the assertion is dead code
func TestInvalidConfig(t *testing.T) {
    var buf bytes.Buffer
    logger := slog.New(slog.NewJSONHandler(&buf, nil))
    serverConfigFromEnv(logger) // expected to emit a WARN
    // buf.String() is never checked — the test always passes
}

// CORRECT — assert the buffer contains (or does not contain) expected content
func TestInvalidConfig(t *testing.T) {
    var buf bytes.Buffer
    logger := slog.New(slog.NewJSONHandler(&buf, nil))
    serverConfigFromEnv(logger)
    if !strings.Contains(buf.String(), "invalid SHUTDOWN_TIMEOUT") {
        t.Errorf("expected warn log, got: %q", buf.String())
    }
}
```

The symmetric assertion also matters: when a test expects **no** log output,
assert `buf.Len() == 0`. A missing negative assertion allows future log
additions to silently appear in test output without failing CI:

```go
if buf.Len() > 0 {
    t.Errorf("expected no log output, got: %q", buf.String())
}
```

**Reference:** `backend/cmd/server/main_test.go` — `TestServerConfigFromEnv`
checks both directions: `strings.Contains(buf.String(), tc.wantLog)` for the
warning cases and `buf.Len() > 0` for the silent cases.

**Sister rule:** [`t.Parallel()` + `slog.SetDefault()` mutation is a data race](tparallel-slog-setdefault-race.md)
