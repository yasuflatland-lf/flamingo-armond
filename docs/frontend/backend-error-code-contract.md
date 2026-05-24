# Backend error-code contract

> Part of [`frontend/CLAUDE.md`](../../frontend/CLAUDE.md). See the index for related chapters.

The backend (`backend/internal/gqlerr`) returns five `extensions.code` values. The frontend handles them at two layers:

| Code | Backend meaning | Frontend action |
|---|---|---|
| `BAD_USER_INPUT` | Validation failure; `extensions.field` names the form field | Show inline field error via `getBackendFieldErrors` |
| `UNAUTHENTICATED` | No valid session, or cross-user-access on an ownership-protected resource | RSC: `isUnauthenticatedGraphQLError` from `@/lib/apollo/graphql-errors`; client: banner via `getBackendErrorBanner` |
| `FORBIDDEN` | Authenticated but not authorized for the operation (e.g. a non-admin attempting an admin mutation); the message is intentionally generic | RSC: `isForbiddenGraphQLError` from `@/lib/apollo/graphql-errors` — distinct from `UNAUTHENTICATED`, the user has a valid session so the action is a banner/denial, not a redirect to login; client: banner via `getBackendErrorBanner` |
| `INTERNAL` | Server-side error; original message is hidden | Banner via `getBackendErrorBanner` |
| `CANCELLED` | Request aborted before completion — context cancellation or client disconnect (`context.Canceled` / `context.DeadlineExceeded`); logged at WARN, no user-facing action expected | No dedicated handler; if surfaced, falls through to the generic banner via `getBackendErrorBanner` |

`getBackendErrorBanner` scans all errors in priority order (INTERNAL > UNAUTHENTICATED > first non-field error) rather than relying on array position. The backend collapses `NOT_FOUND` into `UNAUTHENTICATED` for ownership-protected resources — the frontend does not need a separate "not found" UI branch on those routes.

