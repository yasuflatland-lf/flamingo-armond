# Two-tier API pattern: open primitive + strict/typed wrapper

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.

Several places in the backend expose a shared shape: an **open, general-purpose primitive** that any caller can use for new or one-off cases, paired with a **strict or typed wrapper** that bakes the conventional decision (a specific error message, a fixed extension key, an all-or-nothing contract) into one place. The wrapper exists so the call site is compile-checked and so the convention lives in exactly one location; the primitive exists so new variants do not have to extend the wrapper before they can be expressed. The two worked examples below show the pattern in two unrelated subsystems.

## Worked example: `gqlerr` open helper + typed wrapper

When a `BAD_USER_INPUT` error needs to carry a structured payload beyond the standard `{code, field}` envelope (e.g. an existing entity's ID and a preview field for the frontend to display), use two layers:

1. **Generic open helper** — `BadUserInputWithExtensions(field, message string, extra map[string]any)` accepts any extension map. Use it for new one-off cases.
2. **Domain-specific typed wrapper** — once a call site stabilises, wrap the generic helper in a named function so the call site is compile-checked and the extension keys are assembled in one place.
3. **Typed reason constant** — pair the wrapper with a typed string constant so the discriminator string the frontend branches on lives in exactly one place and is never stringly-typed at the call site.

The frontend discriminates on `extensions.reason` (a sub-key), not on `extensions.code`. This keeps the top-level `code` as the coarse class (`BAD_USER_INPUT`) so existing `IsCode` matchers and field-error UI keep working without modification.

`BadUserInputWithExtensions` is the current primitive for this pattern (see `backend/internal/gqlerr/errors.go`). When domain variants are instead representable as schema-level union members (e.g. `createCard` returns `CreateCardResult = CreateCardSuccess | CardDuplicateFrontError`), prefer the union approach — the `__typename` discriminator is checked by the client directly, so no extensions-based reason constant is needed.

## Worked example: optional env-config reader + strict wrapper

When a feature's env vars are required *as a group* but the feature itself is optional (server should still boot if the group is absent), provide two readers:

1. **Optional reader** — `OptionalConfigFromEnv() (cfg, missing []string, err error)`. Returns the missing var names as a slice when any are blank, returns an error only for *malformed* values (e.g. all-comma CSV) that should fail startup regardless. The startup path branches on `len(missing) > 0` to disable the feature gracefully and log the missing names.
2. **Strict wrapper** — `ConfigFromEnv() (cfg, error)`. A thin wrapper over the optional reader that converts the first missing var into an error matching the legacy single-var message format. Tests and consumers that need the all-or-nothing contract use this; the wrapper is the one place the "missing means error" decision lives.

**Why two:** the startup path needs to log the missing list and skip route registration (fail-soft); existing tests need the exact pre-existing error message. Splitting the two callers across two readers lets each evolve independently while keeping the source-of-truth in the optional reader.

**Determinism:** the optional reader iterates a package-level `varOrder` slice (not a map) so the missing list and the wrapper's "first missing" message are stable across map-iteration randomness.

**Reference:** `notionsync.OptionalConfigFromEnv` and `notionsync.ConfigFromEnv` in `backend/internal/handler/notionsync/config.go`.
