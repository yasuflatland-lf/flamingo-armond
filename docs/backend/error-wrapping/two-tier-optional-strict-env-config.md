# Two-tier env config API: optional reader + strict wrapper

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.

When a feature's env vars are required *as a group* but the feature itself is optional (server should still boot if the group is absent), provide two readers:

1. **Optional reader** — `OptionalConfigFromEnv() (cfg, missing []string, err error)`. Returns the missing var names as a slice when any are blank, returns an error only for *malformed* values (e.g. all-comma CSV) that should fail startup regardless. The startup path branches on `len(missing) > 0` to disable the feature gracefully and log the missing names.
2. **Strict wrapper** — `ConfigFromEnv() (cfg, error)`. A thin wrapper over the optional reader that converts the first missing var into an error matching the legacy single-var message format. Tests and consumers that need the all-or-nothing contract use this; the wrapper is the one place the "missing means error" decision lives.

**Why two:** the startup path needs to log the missing list and skip route registration (fail-soft); existing tests need the exact pre-existing error message. Splitting the two callers across two readers lets each evolve independently while keeping the source-of-truth in the optional reader.

**Determinism:** the optional reader iterates a package-level `varOrder` slice (not a map) so the missing list and the wrapper's "first missing" message are stable across map-iteration randomness.

**Reference:** `notionsync.OptionalConfigFromEnv` and `notionsync.ConfigFromEnv` in `backend/internal/handler/notionsync/config.go`.
