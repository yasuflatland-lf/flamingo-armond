---
paths:
  - "frontend/**"
---

# Frontend RSC error handling

> Applies to: `frontend/src/app/**/*.tsx` (React Server Components, layouts, route handlers) and `frontend/src/lib/apollo/server.ts` (the `gqlFetch` helper). Cross-cutting because every RSC that talks to Supabase + GraphQL has the same five failure modes — session-missing, session-expired, GraphQL `UNAUTHENTICATED`, GraphQL `FORBIDDEN`, and infrastructure 5xx — and they must be handled uniformly to avoid 500-ing the whole app.

## `AuthSessionMissingError` is the "no session" signal — filter it, do not throw

`createSupabaseServerClient().auth.getUser()` returns `{ data: { user: null }, error: AuthSessionMissingError }` for an anonymous request. This is the **happy path** for landing pages, the public Header, and any RSC that renders something for both signed-in and anonymous viewers. Treat any other error as a real failure:

```ts
const { data: { user }, error } = await supabase.auth.getUser();
if (error && error.name !== "AuthSessionMissingError") {
  console.error("[scope] getUser() failed:", error.name, error.message);
  throw error;
}
// `user` is `User | null` here — branch on it.
```

The filter is keyed on `error.name` (string), not on `instanceof` — Supabase's class identity does not survive serialization across the SDK's internal boundaries reliably. The pattern applies to every caller of `supabase.auth.getUser()`. `getUser()` is called only by the middleware (`frontend/src/lib/supabase/middleware.ts`). Every other protected page does **not** call `getUser()` — it reads the middleware-forwarded `x-auth-status` header via `readAuthContext(await headers())` (`frontend/src/lib/supabase/auth-status.ts`) and redirects when the status is not `authenticated`. Most pages redirect to `/login`; the admin pages (`app/admin/layout.tsx`, `app/admin/users/page.tsx`) redirect to `/` instead; and `app/login/page.tsx` inverts the check — an `authenticated` status redirects the visitor away to `/cardgroups`, while `anonymous`, `stale`, and `error` statuses all render the login form:

```ts
import { headers } from "next/headers";
import { readAuthContext } from "@/lib/supabase/auth-status";
// ...
if (readAuthContext(await headers()).status !== "authenticated") redirect("/login");
```

To verify the current set of direct `getUser()` callers: `grep -rn "auth.getUser\|auth.getSession\|auth.getClaims" frontend/src/`. The result set is the middleware plus any Apollo server/link helpers that call `getSession()`. Any new file in those results that calls `auth.getUser()` during server render must include the `AuthSessionMissingError` filter; the `getSession()` hits in the Apollo helpers (`lib/apollo/server.ts`, `lib/apollo/auth-link.ts`) fetch the bearer token rather than gate on identity, and the filter does not apply to them.

## Header (root layout) MUST degrade on failure, never throw

The `AppShell` component (`frontend/src/components/nav/app-shell.tsx`) is mounted in `app/layout.tsx` (via the client `ConditionalShell` wrapper, `frontend/src/components/conditional-shell.tsx`, which mounts it on full-shell routes), so anything it throws escapes every route segment's `error.tsx` and surfaces as `app/global-error.tsx` (or Next's default 500 page) — `ConditionalShell` adds no error boundary. The degradation logic lives in the Next.js middleware (`frontend/src/lib/supabase/middleware.ts`), which is the single source of identity. The layout does **no** auth I/O at render time: it reads the middleware-forwarded request headers via `readAuthContext` (`frontend/src/lib/supabase/auth-status.ts`) and passes `user` and `isAdmin` as props down through `ConditionalShell` to `AppShell`. The middleware classifies failures into an `x-auth-status` value (`authenticated` / `anonymous` / `stale` / `error`) and **fails closed** — a dropped or malformed header degrades to the anonymous shell, never a leaked authenticated view. The layout + shell combination is the **display layer** and must never throw; the middleware enforces this by:

1. Classifying a `getUser()` failure that is not an ignorable/anonymous error into `x-auth-status: stale` or `x-auth-status: error` (a degraded, logo-only shell), instead of throwing.
2. Resolving `isAdmin` from the verified JWT claims (`getClaims()`), wrapped in `try`/`catch` — `getClaims()` can throw non-`AuthError` exceptions, so the middleware swallows them, `console.warn`s, and falls back to `isAdmin = false`. `isAdmin` is a UI hint only.

The **authorization gate** is `app/admin/layout.tsx`, which redirects on failure. The two layers are deliberately asymmetric: Header is UI hint, admin layout is the enforcement boundary. A broken Header never grants access; an over-eager Header throw 500s the whole site.

This is why `app/<route>/error.tsx` cannot rescue layout-level throws — the same constraint as [`docs/frontend/gotchas-encountered.md`](../../docs/frontend/gotchas-encountered.md) ("`app/<route>/error.tsx` does not catch errors from `app/layout.tsx`").

## Structurally parse GraphQL `extensions.code` — never substring-match the message

`gqlFetch` (`frontend/src/lib/apollo/server.ts`) handles two distinct response shapes depending on whether `json.data` is present:

1. **No data** (`json.errors` set AND `json.data == null`) — throws an error:
   ```
   Error("GraphQL errors: " + JSON.stringify(json.errors))
   ```
   i.e. the literal prefix `GraphQL errors: ` followed by the JSON-stringified `errors` array as returned by the backend. Each entry has the standard `{ message, path, extensions: { code, ... } }` shape.

2. **Partial response** (`json.errors` set AND `json.data != null`, per GraphQL over HTTP §5.2) — returns `json.data` and emits a console warning. **Exception**: if any error has `extensions.code` of `UNAUTHENTICATED` or `FORBIDDEN`, the error is re-thrown to preserve the auth-redirect contract — callers using `isUnauthenticatedGraphQLError` or `isForbiddenGraphQLError` continue to work correctly.

Code that needs to branch on a specific error code (e.g. swallowing `UNAUTHENTICATED` in the Header, or distinguishing `UNAUTHENTICATED` from a real failure on a HomePage redirect) MUST parse the JSON and read `extensions.code`, not substring-match the message. Substring matches conflate a real `UNAUTHENTICATED` extension with any error whose message text happens to contain the word, including user-supplied input echoed by the backend, future telemetry strings, or stack-trace fragments.

Use the shared helper `isUnauthenticatedGraphQLError` from `@/lib/apollo/graphql-errors` — do **not** re-inline the JSON-parse logic at each call site. Lifting it to one module ensures every consumer applies the same `prefix.startsWith` check, the same JSON shape assumption, and the same `try/catch` for malformed payloads. The same structural check also runs inside `gqlFetch` for partial-response auth errors, ensuring consistent classification. Today's call sites include `frontend/src/app/page.tsx`, `frontend/src/app/cards/new/page.tsx`, `frontend/src/app/cardgroups/page.tsx`, `frontend/src/app/profile/page.tsx`, and several others — run `grep -rln 'isUnauthenticatedGraphQLError' frontend/src/` to get the current list.

```ts
import { isUnauthenticatedGraphQLError } from "@/lib/apollo/graphql-errors";

try {
  data = await gqlFetch(MyQuery, { revalidate: 0 });
} catch (err) {
  if (isUnauthenticatedGraphQLError(err)) redirect("/login");
  // ...other branches
}
```

Use `isUnauthenticatedGraphQLError` for all code paths — including simple redirect-only cases — as it performs a structural `extensions.code` check rather than substring matching and avoids false positives from user-supplied content or stack traces. When `UNAUTHENTICATED` and `FORBIDDEN` collapse to the same outcome, combine both helpers in one guard — `if (isUnauthenticatedGraphQLError(err) || isForbiddenGraphQLError(err)) redirect("/")` — as `app/admin/layout.tsx` does, where a logged-out session and a non-admin role both redirect to `/`. Where the two codes diverge in user-facing copy, keep them split (see [`docs/frontend/rsc-error-handling/unauthenticated-vs-forbidden-message-asymmetry.md`](../../docs/frontend/rsc-error-handling/unauthenticated-vs-forbidden-message-asymmetry.md)).

## Detailed cases (on-demand)

- [Partial-response errors in `gqlFetch`: auth codes re-throw, others return data + warn](../../docs/frontend/rsc-error-handling/partial-response-gqlfetch.md)
- [Schema non-null does not protect against partial-response null-bubble — guard at the consumer](../../docs/frontend/rsc-error-handling/partial-response-non-null-bubble-guard.md)
- [Use `CombinedGraphQLErrors.is(err)` — never `instanceof CombinedGraphQLErrors`](../../docs/frontend/rsc-error-handling/combinedgraphqlerrors-is-over-instanceof.md)
- [`getBackendErrorBanner` deliberately skips field-level `BAD_USER_INPUT` — use `getBackendFieldErrors` first](../../docs/frontend/rsc-error-handling/getbackenderrorbanner-skips-field-level.md)
- [Structural error parsers must `console.warn` (not silently `continue`) when the shape narrows wrong](../../docs/frontend/rsc-error-handling/structural-error-parsers-warn-on-shape-narrow.md)
- [`console.error("[scope] getUser() failed:", err.name, err.message)` before throwing in RSC](../../docs/frontend/rsc-error-handling/console-error-scope-prefix-before-throwing.md)
- [Skip auth-requiring GraphQL calls when the client knows the user is anonymous](../../docs/frontend/rsc-error-handling/skip-auth-graphql-calls-when-anonymous.md)
- [Pair every `try { ... } finally { setLoading(false) }` with a `catch` for transport rejections](../../docs/frontend/rsc-error-handling/pair-try-finally-with-catch-for-transport.md)
- [Substring-matching SDK error strings: pair mapped copy with raw-message warn for unmapped paths](../../docs/frontend/rsc-error-handling/substring-matching-sdk-error-strings.md)
- [`revalidate: 0` for any RSC fetch that depends on the current user](../../docs/frontend/rsc-error-handling/revalidate-zero-for-auth-dependent-rsc-fetch.md)
- [Pages that bypass `AppShell` MUST render their own `<main>` landmark](../../docs/frontend/rsc-error-handling/pages-bypassing-appshell-must-render-main.md)
- [Initial-load UNAUTHENTICATED redirects belong in the RSC `page.tsx`, not in the client `error.tsx`](../../docs/frontend/rsc-error-handling/initial-load-unauthenticated-redirects-in-rsc.md)
- [Mid-session UNAUTHENTICATED in a client component: degraded banner with `<Link href="/login">`, not `redirect()`](../../docs/frontend/rsc-error-handling/mid-session-unauthenticated-degraded-banner.md)
- [Redact `err.message` from structured `console` payloads when the upstream may carry user content](../../docs/frontend/rsc-error-handling/redact-err-message-from-console-payloads.md)
- [Empty result is a business state, not an error — render an empty-state UI, never throw](../../docs/frontend/rsc-error-handling/empty-result-is-business-state-not-error.md)
- [Auth check runs outside the Suspense boundary — data fetch runs inside](../../docs/frontend/rsc-error-handling/auth-outside-suspense-boundary.md)
- [GraphQL UNAUTHENTICATED must redirect to `/login`, not to an auth-required route](../../docs/frontend/rsc-error-handling/unauthenticated-redirect-target-must-be-login.md)
- [Apollo `client.query()` is not cancelled on unmount — guard `setState` with `isMountedRef`](../../docs/frontend/rsc-error-handling/apollo-client-query-unmount-guard.md)
- [`supabase.auth.getClaims()` has a three-way return AND can throw non-AuthError exceptions — use try/catch outside an error boundary (e.g. middleware)](../../docs/frontend/rsc-error-handling/getclaims-three-way-return.md)
- [`isUnauthenticatedGraphQLError` matches gqlFetch — not Apollo Client runtime errors](../../docs/frontend/rsc-error-handling/apollo-runtime-vs-gqlfetch-error-shape.md)
- [A failed `void refetch()` surfaces via the `useQuery` hook's `error` state — no local transport-error machinery needed](../../docs/frontend/rsc-error-handling/apollo-v4-refetch-rejection-and-hook-error.md)
- [`useMutation` rejects while `useLazyQuery` resolves — `result.error` after a mutation is dead code](../../docs/frontend/rsc-error-handling/mutate-rejects-while-lazyquery-resolves.md)
- [Fire-and-forget mutation: structured warn for null payload, non-success variant, and rejection](../../docs/frontend/rsc-error-handling/fire-and-forget-mutation-warn-on-null-and-non-success.md)
- [Refetch after a mutation success: isolate it from the error-classification catch, and match the `refetchQueries` predicate on identity only](../../docs/frontend/rsc-error-handling/refetch-after-mutation-success-isolation.md)
- [`UNAUTHENTICATED` collapses to generic copy; `FORBIDDEN` preserves the server's specific reason](../../docs/frontend/rsc-error-handling/unauthenticated-vs-forbidden-message-asymmetry.md)
- [Owner-gated mutation: no FORBIDDEN-specific banner — the route gate makes the usecase owner-check a backstop](../../docs/frontend/rsc-error-handling/owner-gated-mutation-no-forbidden-banner.md)
- [Drop the backend not-found message at the client boundary — render localized copy, carry no `message` in the outcome](../../docs/frontend/rsc-error-handling/drop-backend-not-found-message-at-client.md)
- [Suspense fallback does not catch thrown errors — wrap async server components in try/catch](../../docs/frontend/rsc-error-handling/suspense-does-not-catch-thrown-errors.md)
