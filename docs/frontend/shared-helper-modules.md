# Shared helper modules

> Part of [`frontend/CLAUDE.md`](../../frontend/CLAUDE.md). See the index for related chapters.

Import these instead of re-inlining the logic:

| Import | Contract |
|---|---|
| `getBackendFieldErrors(err)` from `@/lib/apollo/errors` | Returns `{ fieldName: message }` for `BAD_USER_INPUT` errors with a `field` extension |
| `getBackendErrorBanner(err)` from `@/lib/apollo/errors` | Returns a user-facing banner string for non-field errors; priority INTERNAL > UNAUTHENTICATED > first non-field |
| `isUnauthenticatedGraphQLError(err)` from `@/lib/apollo/graphql-errors` | Structurally parses `extensions.code` and returns `true` for `UNAUTHENTICATED`; safe to call from both Server and Client Components |
| `redirectIfAuthError(err, target, { forbidden? })` from `@/lib/apollo/graphql-errors` | The RSC `catch`-arm classifier: redirects to `target` on `UNAUTHENTICATED`, and also on `FORBIDDEN` when `forbidden` is set. Owns only the redirect decision — logging, rethrow and degrade-to-default stay at the call site |
| `requireAuthenticated(target)` from `@/lib/supabase/auth-status` | The RSC auth gate: reads the middleware-forwarded `x-auth-status` header and redirects to `target` unless the request is `authenticated`; returns the `AuthContext` for pages that also need `email` / `isAdmin`. Call it outside any `<Suspense>` boundary |
| `formatMediumDate(iso)` from `@/lib/format` | Formats an ISO date string as a locale-aware medium-length date (e.g. "Jun 15, 2024") |
| `<FieldError zodErrors backendError />` from `@/lib/forms/field-error` | Renders the first Zod issue message or the fallback `backendError` string as a destructive `<p>` |
| `graphemeCount(s)` from `@/schemas/grapheme` | Counts UAX #29 grapheme clusters via `Intl.Segmenter`; shared by all schema length validators |
| `safeDecodePathSegment(segment)` from `@/lib/safe-decode-path-segment` | Decodes a percent-encoded `usePathname()` path segment; returns `null` on `URIError` (malformed `%XX`) instead of throwing. Layout-level nav components MUST handle the `null` return to prevent crashing the root layout — see [`docs/frontend/typescript-conventions/usepathname-returns-percent-encoded.md`](typescript-conventions/usepathname-returns-percent-encoded.md). Safe to import from both Server and Client Components. |

