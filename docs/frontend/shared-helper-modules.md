# Shared helper modules

> Part of [`docs/frontend.md`](../frontend.md). See the index for related chapters.

Import these instead of re-inlining the logic:

| Import | Contract |
|---|---|
| `getBackendFieldErrors(err)` from `@/lib/apollo/errors` | Returns `{ fieldName: message }` for `BAD_USER_INPUT` errors with a `field` extension |
| `getBackendErrorBanner(err)` from `@/lib/apollo/errors` | Returns a user-facing banner string for non-field errors; priority INTERNAL > UNAUTHENTICATED > first non-field |
| `redirectIfUnauthenticated(err, target)` from `@/lib/apollo/server-redirect` | RSC only (`server-only`). Redirects to `target` on `UNAUTHENTICATED`; rethrows all other errors. Returns `never` |
| `formatMediumDate(iso)` from `@/lib/format` | Formats an ISO date string as a locale-aware medium-length date (e.g. "Jun 15, 2024") |
| `<FieldError zodErrors backendError />` from `@/lib/forms/field-error` | Renders the first Zod issue message or the fallback `backendError` string as a destructive `<p>` |
| `graphemeCount(s)` from `@/schemas/grapheme` | Counts UAX #29 grapheme clusters via `Intl.Segmenter`; shared by all schema length validators |

