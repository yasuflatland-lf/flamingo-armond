# Two-tier API pattern: open primitive + strict/typed wrapper

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.

Several places in the backend expose a shared shape: an **open, general-purpose primitive** for one-off cases, paired with a **strict or typed wrapper** that bakes the conventional decision (a specific error message, a fixed extension key, an all-or-nothing contract) into one place. The wrapper compile-checks the call site and keeps the convention in exactly one location; the primitive lets new variants land without extending the wrapper first. The worked example below describes the shape using the env-config reader; only its optional half currently exists in the tree.

## Worked example: optional env-config reader + strict wrapper

When a feature's env vars are required *as a group* but the feature itself is optional (server should still boot if the group is absent), provide an optional reader, plus a strict wrapper only when a caller needs one:

1. **Optional reader** — `OptionalConfigFromEnv() (cfg, missing []string, err error)`. Returns the missing var names as a slice when any are blank, returns an error only for *malformed* values (e.g. all-comma CSV) that should fail startup regardless. The startup path branches on `len(missing) > 0` to disable the feature gracefully and log the missing names.
2. **Strict wrapper** — a `(cfg, error)` function that converts the first missing var into an error. Add it only when a caller needs the all-or-nothing contract; the notionsync package had one for a legacy error message and deleted it once its last caller (a test) was folded into the optional reader's tests.

**Why two:** the startup path needs to log the missing list and skip route registration (fail-soft), while an all-or-nothing consumer would want a single error. Keep the optional reader as the source of truth and add the strict wrapper only when such a consumer exists.

**Determinism:** the optional reader iterates a package-level `varOrder` slice (not a map) so the missing list is stable across map-iteration randomness.

**Reference:** `notionsync.OptionalConfigFromEnv` in `backend/internal/handler/notionsync/config.go` (the optional reader; no strict wrapper exists in the tree at present).
