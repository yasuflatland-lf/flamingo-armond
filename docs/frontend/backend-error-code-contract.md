# Backend error-code contract

> Part of [`frontend/CLAUDE.md`](../../frontend/CLAUDE.md). See the index for related chapters.

The backend (`backend/internal/gqlerr`) returns three `extensions.code` values. The frontend handles them at two layers:

| Code | Backend meaning | Frontend action |
|---|---|---|
| `BAD_USER_INPUT` | Validation failure; `extensions.field` names the form field | Show inline field error via `getBackendFieldErrors` |
| `UNAUTHENTICATED` | No valid session, or cross-user-access on an ownership-protected resource | RSC: `isUnauthenticatedGraphQLError` from `@/lib/apollo/graphql-errors`; client: banner via `getBackendErrorBanner` |
| `INTERNAL` | Server-side error; original message is hidden | Banner via `getBackendErrorBanner` |

`getBackendErrorBanner` scans all errors in priority order (INTERNAL > UNAUTHENTICATED > first non-field error) rather than relying on array position. The backend collapses `NOT_FOUND` into `UNAUTHENTICATED` for ownership-protected resources — the frontend does not need a separate "not found" UI branch on those routes.

