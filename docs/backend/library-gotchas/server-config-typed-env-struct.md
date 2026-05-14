# Consolidate env-var reads into a typed config struct

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

Scattered `os.Getenv()` calls inline in `run()` make it hard to see which
environment variables the server reads, hard to test each parsing branch in
isolation, and easy to introduce duplicate fallback logic. Consolidate all
env-var parsing into a typed struct + factory function:

```go
const defaultShutdownTimeout = 25 * time.Second

type serverConfig struct {
    shutdownTimeout time.Duration
}

// serverConfigFromEnv builds a serverConfig from environment variables.
// SHUTDOWN_TIMEOUT accepts any value accepted by time.ParseDuration; invalid
// or non-positive values fall back to defaultShutdownTimeout with a WARN log.
func serverConfigFromEnv(logger *slog.Logger) serverConfig {
    shutdownDur := defaultShutdownTimeout
    if v := os.Getenv("SHUTDOWN_TIMEOUT"); v != "" {
        d, err := time.ParseDuration(v)
        if err != nil {
            logger.Warn("invalid SHUTDOWN_TIMEOUT, using default",
                "value", v, "err", err, "default", defaultShutdownTimeout.String())
        } else if d <= 0 {
            logger.Warn("non-positive SHUTDOWN_TIMEOUT, using default",
                "value", v, "default", defaultShutdownTimeout.String())
        } else {
            shutdownDur = d
        }
    }
    return serverConfig{shutdownTimeout: shutdownDur}
}
```

**Benefits of this pattern:**

- **Visibility.** All configuration surface area is visible at a glance by
  reading one struct definition.
- **Testability.** Each parsing branch is a simple table-driven test that calls
  `serverConfigFromEnv` with a `t.Setenv`-controlled env var and asserts on the
  returned struct. No full server required.
- **Consistent fallback.** The fallback value and its warning message live in
  one place; inline `os.Getenv()` calls scatter both across `run()`.

**When to apply:** anytime `run()` has two or more env-var reads that participate
in the same logical configuration domain (e.g. timeouts, feature flags, external
endpoints). A single required-or-fatal env var (`PING_TOKEN`) that triggers a
hard error does not need its own struct field — keep that inline.

**Reference:** `backend/cmd/server/main.go` `serverConfig` + `serverConfigFromEnv`;
tested by `TestServerConfigFromEnv` in `backend/cmd/server/main_test.go`.

**Sister rules:**
- [Extract startup helpers to make branch coverage testable without a live server](extract-startup-helpers-for-branch-coverage.md)
- [`t.Setenv` vs `os.Setenv` in tests](tsetenv-vs-os-setenv.md)
