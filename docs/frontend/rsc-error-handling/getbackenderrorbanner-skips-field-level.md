# `getBackendErrorBanner` deliberately skips field-level `BAD_USER_INPUT` — use `getBackendFieldErrors` first

> Part of the [frontend RSC error handling](../../../.claude/rules/frontend-rsc-error-handling.md) rules.

`getBackendErrorBanner` (`frontend/src/lib/apollo/errors.ts`) returns `undefined` for `BAD_USER_INPUT` errors that carry an `extensions.field`, because those errors are meant to be displayed inline next to the offending field, not in a generic banner. A call site that passes such an error to `getBackendErrorBanner` and displays the result will silently show nothing.

Any UI flow that wants to surface a field-level error inline (instead of a generic banner) MUST explicitly call `getBackendFieldErrors(err)?.<field>` first, then fall back to `getBackendErrorBanner`, then to a generic copy string:

```ts
// correct: check field-level error first, then banner, then generic fallback
const fieldErrors = getBackendFieldErrors(err);
const banner = getBackendErrorBanner(err);
setErrorMessage(
  fieldErrors?.front ??    // field-level inline message
  banner ??                // generic banner (skipped for field-level BAD_USER_INPUT)
  "An unexpected error occurred",
);
```

The three-way fallback ensures every typed error from the backend reaches the UI at the most specific level available, without duplicating the field-level message in a second banner. Reference: `frontend/src/app/cards/new/cards-new-client.tsx` `handleOverwrite`.
