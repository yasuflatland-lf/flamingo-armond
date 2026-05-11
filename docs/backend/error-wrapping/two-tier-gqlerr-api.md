# Two-tier `gqlerr` API: generic open helper + domain-specific typed wrapper

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.

When a `BAD_USER_INPUT` error needs to carry a structured payload beyond the standard `{code, field}` envelope (e.g. an existing entity's ID and a preview field for the frontend to display), use two layers:

1. **Generic open helper** — `BadUserInputWithExtensions(field, message string, extra map[string]any)` accepts any extension map. Use it for new one-off cases.
2. **Domain-specific typed wrapper** — once a call site stabilises, wrap the generic helper in a named function so the call site is compile-checked and the extension keys are assembled in one place.
3. **Typed reason constant** — pair the wrapper with a typed string constant so the discriminator string the frontend branches on lives in exactly one place and is never stringly-typed at the call site.

The frontend discriminates on `extensions.reason` (a sub-key), not on `extensions.code`. This keeps the top-level `code` as the coarse class (`BAD_USER_INPUT`) so existing `IsCode` matchers and field-error UI keep working without modification.

`BadUserInputWithExtensions` is the current primitive for this pattern (see `backend/internal/gqlerr/errors.go`). When domain variants are instead representable as schema-level union members (e.g. `createCard` returns `CreateCardResult = CreateCardSuccess | CardDuplicateFrontError`), prefer the union approach — the `__typename` discriminator is checked by the client directly, so no extensions-based reason constant is needed.
