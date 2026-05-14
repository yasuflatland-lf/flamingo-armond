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

The filter is keyed on `error.name` (string), not on `instanceof` — Supabase's class identity does not survive serialization across the SDK's internal boundaries reliably. The pattern applies to every RSC, layout, route handler, or server action that calls `supabase.auth.getUser()`. To find all current call sites: `grep -rn "auth.getUser\|auth.getSession\|auth.getClaims" frontend/src/`. Any new file added to those results must include the filter — there is no per-file list to maintain because the list rotted before this entry was rewritten.

## Header (root layout) MUST degrade on failure, never throw

The header component (`GlobalHeader` in `frontend/src/components/nav/global-header.tsx`) lives in `app/layout.tsx`, so anything it throws escapes every route segment's `error.tsx` and surfaces as `app/global-error.tsx` (or Next's default 500 page). Throwing for a failed `me` fetch turns one stale role lookup into a site-wide 500. The Header is the **display layer** and must:

1. Render a degraded shell (logo only) when `getUser()` fails for a reason other than `AuthSessionMissingError`.
2. Skip the GraphQL `me` call entirely when `user == null` — anonymous users do not have roles to load, and the call would spam `UNAUTHENTICATED` into the warn log (see "Skip auth-requiring GraphQL calls" below).
3. On `me` failure, swallow `UNAUTHENTICATED` silently (Supabase session valid + GraphQL token rejected = expected race during clock skew or JWKS rotation), `console.warn` everything else, and fall back to `isAdmin = false`.

The **authorization gate** is `app/admin/layout.tsx`, which redirects on failure. The two layers are deliberately asymmetric: Header is UI hint, admin layout is the enforcement boundary. A broken Header never grants access; an over-eager Header throw 500s the whole site.

This is why `app/<route>/error.tsx` cannot rescue layout-level throws — the same constraint as the `Gotchas encountered` entry in `docs/frontend.md` ("`app/<route>/error.tsx` does not catch errors from `app/layout.tsx`").

## Structurally parse GraphQL `extensions.code` — never substring-match the message

`gqlFetch` (`frontend/src/lib/apollo/server.ts`) handles two distinct response shapes depending on whether `json.data` is present:

1. **No data** (`json.errors` set AND `json.data == null`) — throws an error:
   ```
   Error("GraphQL errors: " + JSON.stringify(json.errors))
   ```
   i.e. the literal prefix `GraphQL errors: ` followed by the JSON-stringified `errors` array as returned by the backend. Each entry has the standard `{ message, path, extensions: { code, ... } }` shape.

2. **Partial response** (`json.errors` set AND `json.data != null`, per GraphQL over HTTP §5.2) — returns `json.data` and emits a console warning. **Exception**: if any error has `extensions.code` of `UNAUTHENTICATED` or `FORBIDDEN`, the error is re-thrown to preserve the auth-redirect contract — callers using `isUnauthenticatedGraphQLError` or `isForbiddenGraphQLError` continue to work correctly.

Code that needs to branch on a specific error code (e.g. swallowing `UNAUTHENTICATED` in the Header, or distinguishing `UNAUTHENTICATED` from a real failure on a HomePage redirect) MUST parse the JSON and read `extensions.code`, not substring-match the message. Substring matches conflate a real `UNAUTHENTICATED` extension with any error whose message text happens to contain the word, including user-supplied input echoed by the backend, future telemetry strings, or stack-trace fragments.

Use the shared helper `isUnauthenticatedGraphQLError` from `@/lib/apollo/graphql-errors` — do **not** re-inline the JSON-parse logic at each call site. Lifting it to one module ensures every consumer applies the same `prefix.startsWith` check, the same JSON shape assumption, and the same `try/catch` for malformed payloads. The same structural check also runs inside `gqlFetch` for partial-response auth errors, ensuring consistent classification. Today's call sites: `frontend/src/components/nav/global-header.tsx`, `frontend/src/app/page.tsx`, and `frontend/src/app/cards/new/page.tsx`.

```ts
import { isUnauthenticatedGraphQLError } from "@/lib/apollo/graphql-errors";

try {
  data = await gqlFetch(MyQuery, { revalidate: 0 });
} catch (err) {
  if (isUnauthenticatedGraphQLError(err)) redirect("/login");
  // ...other branches
}
```

The older `redirectIfUnauthenticated` helper (in `frontend/src/lib/apollo/server-redirect.ts`) still uses a substring match and is grandfathered for legacy redirect-only sites where both sides converge on the same `/login` target. **New** code paths — silent swallow vs. warn vs. redirect, anything that branches finer than "redirect on auth failure" — MUST use the structural helper.

## Detailed cases (on-demand)

- [Partial-response errors in `gqlFetch`: auth codes re-throw, others return data + warn](../../docs/frontend/rsc-error-handling/partial-response-gqlfetch.md)
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
- [`supabase.auth.getClaims()` has a three-way return — branch on `claimsData == null`](../../docs/frontend/rsc-error-handling/getclaims-three-way-return.md)
