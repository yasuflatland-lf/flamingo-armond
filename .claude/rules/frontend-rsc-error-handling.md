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

`gqlFetch` (`frontend/src/lib/apollo/server.ts`) throws a single message-shaped error when the backend returns GraphQL errors:

```
Error("GraphQL errors: " + JSON.stringify(json.errors))
```

i.e. the literal prefix `GraphQL errors: ` followed by the JSON-stringified `errors` array as returned by the backend. Each entry has the standard `{ message, path, extensions: { code, ... } }` shape.

Code that needs to branch on a specific code (e.g. swallowing `UNAUTHENTICATED` in the Header, or distinguishing `UNAUTHENTICATED` from a real failure on a HomePage redirect) MUST parse the JSON and read `extensions.code`, not substring-match the message. Substring matches conflate a real `UNAUTHENTICATED` extension with any error whose message text happens to contain the word, including user-supplied input echoed by the backend, future telemetry strings, or stack-trace fragments.

Use the shared helper `isUnauthenticatedGraphQLError` from `@/lib/apollo/graphql-errors` — do **not** re-inline the JSON-parse logic at each call site. Lifting it to one module ensures every consumer applies the same `prefix.startsWith` check, the same JSON shape assumption, and the same `try/catch` for malformed payloads. Today's call sites: `frontend/src/components/nav/global-header.tsx`, `frontend/src/app/page.tsx`, and `frontend/src/app/cards/new/page.tsx`.

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

## `console.error("[scope] getUser() failed:", err.name, err.message)` before throwing in RSC

Next.js may abbreviate or replace thrown errors in production (the App Router strips error.message in prod and renders a generic "Application error" page unless the error is a `NEXT_REDIRECT` or similar special). Operators triaging a failure see only the boundary log, not the underlying cause. RSCs that rethrow a real `getUser()` failure must log first:

```ts
if (error && error.name !== "AuthSessionMissingError") {
  console.error("[home] getUser() failed:", error.name, error.message);
  throw error;
}
```

The `[scope]` prefix is the route or component name (`[home]`, `[login]`, `[global-header]`, `[cards-new]`, `[cardgroups-new]`, `[healthz]`) so the log is greppable. Used today in `frontend/src/app/page.tsx`, `frontend/src/app/login/page.tsx`, `frontend/src/components/nav/global-header.tsx`, `frontend/src/app/cards/new/page.tsx`, `frontend/src/app/cardgroups/new/page.tsx`, and `frontend/src/app/api/healthz/route.ts`.

## Skip auth-requiring GraphQL calls when the client knows the user is anonymous

When an RSC has already determined `user == null` from `supabase.auth.getUser()`, do not issue any GraphQL query that requires `Authorization`. The backend will return `UNAUTHENTICATED`, the call site has to special-case the error, and the warn log fills with expected-and-uninteresting noise. Gate the call:

```ts
let isAdmin = false;
if (user) {
  try {
    const meData = await gqlFetch(HeaderMeQuery, { revalidate: 0 });
    isAdmin = meData.me?.roles.some((r) => r.name === "admin") ?? false;
  } catch (err) { /* ... */ }
}
```

This is the inverse of the "fail-closed in `gqlFetch`" rule documented in `docs/frontend.md` § "Authorization forwarding in `gqlFetch`": `gqlFetch` itself throws on a session-fetch error rather than silently sending an anonymous request, but **callers** of `gqlFetch` are responsible for not invoking it in the first place when they already know the user is anonymous.

## `revalidate: 0` for any RSC fetch that depends on the current user

Any GraphQL query whose result depends on `Authorization` (role lookups, `me`, owned-resource queries, profile data) must pass `{ revalidate: 0 }` to `gqlFetch`. Caching auth-sensitive data either across users (via Next's data cache key, which does not include the access token) or across role changes (admin role revoked while the cached page is alive) is a correctness bug. Used today in `frontend/src/components/nav/global-header.tsx`, `frontend/src/app/page.tsx`, `frontend/src/app/cards/new/page.tsx`, `frontend/src/app/admin/layout.tsx`, `frontend/src/app/profile/page.tsx`, and `frontend/src/app/api/healthz/route.ts` (the last one for probe freshness, not auth, but the constant is the same).

The three `revalidate` states are documented in `docs/frontend.md` — `0` means no cache, `false` means cache forever, omitted means Next's default heuristic. Pick `0` for auth-sensitive; never collapse to a `number` default.
