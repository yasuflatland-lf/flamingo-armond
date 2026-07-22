# Redact `err.message` from structured `console` payloads when the upstream may carry user content

> Part of the [frontend RSC error handling](../../../.claude/rules/frontend-rsc-error-handling.md) rules.

Backend GraphQL error messages can echo user-authored content (a card front, a search query, a profile bio) verbatim — the resolver's `gqlerr.BadUserInput` constructors typically assemble the `message` field from input parameters. A `console.warn(...)` or `console.error(...)` payload that includes the raw `err.message` re-leaks that content into operator logs, browser devtools history, and any Sentry-style collector that ingests `console` calls. The rule is to omit `err.message` and log only stable identifiers — `err.name` for the JS class, plus domain context like `cardgroupId` or `endCursor` that operators need for triage:

```ts
// frontend/src/app/learn/[cardgroupId]/learn-client.tsx
.catch((err) => {
  // err.message is omitted — backend messages may echo user-authored content.
  console.warn("[learn] setLastViewedCardgroup failed", {
    cardgroupId,
    name: err instanceof Error ? err.name : "unknown",
  });
});

// frontend/src/app/cardgroups/cardgroups-client.tsx — fetchMore catch
.catch((err) => {
  console.warn("[cardgroups] fetchMore failed", {
    name: err instanceof Error ? err.name : "unknown",
    searchQuery: search,
    endCursor: cursor,
  });
});
```

The `err instanceof Error ? err.name : "unknown"` widening handles non-Error rejections (a plain string thrown from a third-party library, a `Promise.reject(undefined)`) without crashing the log call site. The structured payload includes domain context (`cardgroupId`, `searchQuery`, `endCursor`) so operators can correlate the warn with the request without seeing the message body.

**Why:** the [§ "Substring-matching SDK error strings"](./substring-matching-sdk-error-strings.md) rule permits logging `err.message` on the **unmapped** path — but only when the SDK's error messages are server-generated and verifiably do not echo user input. GraphQL backends are the opposite: every typed-error constructor is free to inline the offending input into the message for clarity. The default posture for backend errors is therefore "redact `err.message`"; the substring-classifier carve-out applies only to SDKs whose contract guarantees server-only message provenance.

**How to apply:** every `console.warn` / `console.error` that catches a backend GraphQL error or a Supabase auth error MUST omit `err.message` from the structured payload. This applies in RSC pages, client components, and middleware:

- **RSC pages** (`page.tsx`): all pages (including login and admin pages) read session status from the middleware-forwarded `x-auth-status` header, and none call `getUser()` directly. The protected surfaces go through the shared gate `requireAuthenticated(target)` (`frontend/src/lib/supabase/auth-status.ts`), which wraps `readAuthContext(await headers())` and redirects; `app/login/page.tsx` and `app/layout.tsx` call `readAuthContext` directly because neither may redirect on a non-`authenticated` status. Pages that call `gqlFetch` must log only `{ name: err instanceof Error ? err.name : "unknown" }` — `err.message` of a GraphQL error can echo user-authored input and must not appear in operator logs.
- **Client components**: same rule applies to `useQuery`, `useMutation`, and imperative `client.query()` / `client.mutate()` catches.
- **Middleware** (`frontend/src/lib/supabase/middleware.ts` and any middleware calling Supabase auth methods): auth-error logs MUST omit `err.message` — log only `{ name: err instanceof Error ? err.name : "unknown" }`. Supabase auth error messages can carry user-identifying content; `getClaims()` is the middleware's only auth call and both of its failure logs apply this rule — the `{data,error}` error branch logs `claimsError.name` and the throw path logs `err.name`, neither logs a message. See [`getclaims-three-way-return.md`](getclaims-three-way-return.md) for the throw path that bypasses the `{data,error}` shape.

Log `name` (typed via the `instanceof` widen) plus domain identifiers. The user-facing banner copy (`getBackendErrorBanner(err)` or `getBackendFieldErrors(err)?.<field>`) is what surfaces the error to the user; that path runs the parsed extension through a server-trusted classifier and is not the redaction concern. Pin the structural shape with `expect.objectContaining({ name: expect.any(String), cardgroupId: ... })` — using a domain-specific key (`cardgroupId`, `endCursor`) as the discriminator per [`docs/frontend/typescript-conventions.md` § "`expect.objectContaining({ message })` is not enough — add a discriminating key"](../typescript-conventions/expect-objectcontaining-message-is-not-enough.md), and explicitly assert that the spy was NOT called with `expect.objectContaining({ message: expect.anything() })` so a future contributor that adds `err.message` "for debugging" surfaces in CI. Reference: `frontend/src/app/learn/[cardgroupId]/learn-client.tsx` (handleSwipe + persist failures), `frontend/src/app/cardgroups/page.tsx` (gqlFetch catch), `frontend/src/app/cardgroups/cardgroups-client.tsx` (fetchMore catch), `frontend/src/app/cardgroups/[id]/cards/cards-client.tsx` (fetchMore + bulk-delete catches), `frontend/src/app/admin/users/admin-users-client.tsx` (fetchMore catch).
