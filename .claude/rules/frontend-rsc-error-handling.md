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

### Use `CombinedGraphQLErrors.is(err)` — never `instanceof CombinedGraphQLErrors`

Apollo Client ships `CombinedGraphQLErrors.is(err)` as the canonical check because `instanceof` is unreliable across module realm boundaries. In a Next.js build, the server bundle and the client bundle each have their own copy of `@apollo/client/errors`; a `CombinedGraphQLErrors` created in one realm does not pass `instanceof` in the other. The `.is()` static method uses a duck-type check (`err?.graphQLErrors != null`) that survives the realm split.

Every helper in `frontend/src/lib/apollo/graphql-errors.ts` uses `.is()` already. Any new helper added to that file — or anywhere else that must detect a `CombinedGraphQLErrors` — must also use `.is()`:

```ts
// AVOID: fails silently when the error originates in a different bundle realm.
if (err instanceof CombinedGraphQLErrors) { ... }

// PREFER: duck-type check, realm-safe.
if (CombinedGraphQLErrors.is(err)) { ... }
```

**How to apply:** grep for `instanceof CombinedGraphQLErrors` before any merge; every hit is a bug. The lint rule does not catch this automatically — it must be verified in code review. Reference: `frontend/src/lib/apollo/graphql-errors.ts` (`tryGetDuplicateCardInfo`).

### `getBackendErrorBanner` deliberately skips field-level `BAD_USER_INPUT` — use `getBackendFieldErrors` first

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

### Structural error parsers must `console.warn` (not silently `continue`) when the shape narrows wrong

A helper like `tryGetDuplicateCardInfo` iterates `err.errors` and skips entries whose extensions do not match the expected discriminator. When the discriminator (`extensions.reason === "CARD_DUPLICATE_FRONT"`) matches but the payload is missing required fields (`existingCardId`, `existingBack`), silently calling `continue` downgrades a partially-valid backend payload to a generic error path with no observable signal. Operators have no way to know a CARD_DUPLICATE_FRONT entry was received but discarded.

Emit a `console.warn` with the raw `extensions` object when the discriminator matches but the shape is wrong:

```ts
if (!ext || ext.code !== "BAD_USER_INPUT" || ext.reason !== "CARD_DUPLICATE_FRONT") continue;
// discriminator matched — now validate required fields
if (typeof existingCardId !== "string" || existingCardId === "") {
  console.warn(
    "[graphql-errors] CARD_DUPLICATE_FRONT entry missing required extension fields",
    { entry: { message: entry.message, extensions: ext } },
  );
  continue;
}
```

**PII review gate:** new extension fields added to any backend error variant appear verbatim in this warn payload. Review every new field against the PII policy before landing — field names like `existingBack` (card content) may carry user-authored text, while `existingCardId` (a UUID) is safe. Reference: `frontend/src/lib/apollo/graphql-errors.ts` (`tryGetDuplicateCardInfo`).

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

## Pair every `try { ... } finally { setLoading(false) }` with a `catch` for transport rejections

Client components that toggle a loading flag around a third-party SDK call typically write `try { await sdk.doThing(); ... } finally { setLoading(false); }`. The `finally` releases the UI lock so the button re-enables, but it does **not** observe the rejection. The Supabase JS pattern is the canonical example: `await supabase.auth.updateUser({ ... })` returns API-level errors via the resolved `{ error }` object, but **transport-level** failures (network unreachable, DNS failure, request timeout) come out as a rejected promise. Without a `catch`, the rejection escapes the React event handler — no banner renders, no log fires, the user just sees the form return to the idle state with no feedback.

```tsx
// AVOID: API errors handled, transport rejections silently lost.
try {
  setLoading(true);
  const { error } = await supabase.auth.updateUser({ email });
  if (error) { setError(classify(error.message)); return; }
  setSuccess(true);
} finally {
  setLoading(false);
}

// PREFER: catch the rejected promise, log the error name only, surface a generic banner.
try {
  setLoading(true);
  const { error } = await supabase.auth.updateUser({ email });
  if (error) { setError(classify(error.message)); return; }
  setSuccess(true);
} catch (err) {
  // Transport-level failure (network, timeout). The API-shaped failure goes via { error } above.
  // Do NOT log err.message — SDK exception messages can echo user-typed input (the email here).
  console.warn("[change-email] updateUser threw:", err instanceof Error ? err.name : "unknown");
  setError("Network error. Please check your connection and try again.");
} finally {
  setLoading(false);
}
```

**Why:** the two failure modes (API-shaped `{ error }` resolve and rejected promise) reach the call site through different channels. A try/finally without a catch handles only the resolve channel; the reject channel surfaces as an unhandled-promise warning at most, with no UI signal. The user is stuck — the form looks idle, the action did nothing, and nothing tells them why.

**How to apply:** every async event handler that wraps a third-party SDK call in `try/finally` for a UI lock release MUST have a paired `catch`. The catch logs `err.name` only (not `err.message` — SDK exceptions can echo user input, including the very value the user typed into the form), and sets a generic user-facing banner. Test the rejection path with `mockRejectedValue(Object.assign(new Error("network down"), { name: "FetchError" }))` and assert both the banner copy AND the structural log call (`expect(consoleWarnSpy).toHaveBeenCalledWith("[scope] action threw:", "FetchError")`) — the structural assertion is what guarantees `err.message` is not in the log payload, per `.claude/rules/frontend-typescript-conventions.md` § "`expect.objectContaining({ message })` is not enough — add a discriminating key". Reference: `frontend/src/app/profile/change-email/change-email-client.tsx` `handleSubmit` (the `catch (err)` arm and test S7 in the sibling test file).

## Substring-matching SDK error strings: pair mapped copy with raw-message warn for unmapped paths

When mapping a third-party SDK's error.message strings to user-facing copy via `.includes(...)` (e.g. classifying Supabase's `"Email rate limit exceeded"` to `"Too many requests..."`), three risks compose:

1. **Echo risk** — an unmapped error falls through to the UI and exposes the raw upstream string, leaking error names, request IDs, or technical wording the user has no context for.
2. **Operator-blind risk** — if the unmapped path silently shows a generic banner, operators get no signal to extend the classifier when a new upstream message starts firing.
3. **PII risk** — the raw message goes into a `console.warn`. Some SDKs (e.g. browser `fetch` exceptions, third-party validators) echo user-typed input verbatim into the message, so blindly logging `err.message` re-leaks the input.

The compliant shape is a classifier returning `string | null` (mapped copy or `null` to mean "unmapped") plus a two-branch call site:

```ts
function classifySupabaseError(message: string): string | null {
  const lower = message.toLowerCase();
  if (lower.includes("rate limit")) return "Too many requests. Please wait a moment and try again.";
  if (lower.includes("already registered")) return "That email address is already in use.";
  return null;
}

const classified = classifySupabaseError(err.message);
if (classified !== null) {
  // Classified: operators know what happened from the user copy + err.name; no raw needed.
  console.warn("[scope] updateUser failed:", err.name);
  setError(classified);
} else {
  // Unmapped: log the raw upstream message so operators can extend classifySupabaseError.
  // PII gate: this is safe ONLY when the upstream's API error messages are server-generated
  // and do not echo user-typed input. Verify per SDK before applying.
  console.warn("[scope] updateUser failed (unmapped):", err.name, err.message);
  setError("Could not send confirmation link. Please try again.");
}
```

The `string | null` shape collapses indirection at the call site — a wrapper return type like `{ userMessage, classified }` was tried and rejected during review because every caller had to read both fields, and the `userMessage` slot duplicated the generic-fallback copy already living at the call site. Returning `null` lets the call site own the generic copy and the unmapped log together.

**Why:** the unmapped path is what catches new upstream message variants. Without a raw-message log there, the classifier silently falls behind every SDK update — the user keeps seeing the generic copy, and operators have no telemetry to know which upstream message was the one that needs a new mapping. Conversely, logging the raw message on the **mapped** path is redundant and adds log noise — the mapped copy + `err.name` are enough for operator triage.

**How to apply:** any classifier that turns SDK strings into user-facing copy returns `string | null`, and the unmapped branch emits a 3-arg `console.warn("[scope] action failed (unmapped):", err.name, err.message)`. Document the PII gate explicitly in a code comment ("Supabase API error messages are server-generated and do not echo user-typed input, so this is PII-safe"). Pair with two tests: one that asserts the mapped path logs `(prefix, err.name)` only (2-arg shape), and one that asserts the unmapped path logs `(prefix, err.name, err.message)` (3-arg shape). The structural-call assertions guarantee no future contributor adds `err.message` to the mapped path's warn (which would re-introduce the leak this rule prevents). Reference: `frontend/src/app/profile/change-email/change-email-client.tsx` (`classifyUpdateUserError` returning `string | null`) and the S3/S5/S6 test cases in the sibling test file.

## `revalidate: 0` for any RSC fetch that depends on the current user

Any GraphQL query whose result depends on `Authorization` (role lookups, `me`, owned-resource queries, profile data) must pass `{ revalidate: 0 }` to `gqlFetch`. Caching auth-sensitive data either across users (via Next's data cache key, which does not include the access token) or across role changes (admin role revoked while the cached page is alive) is a correctness bug. Used today in `frontend/src/components/nav/global-header.tsx`, `frontend/src/app/page.tsx`, `frontend/src/app/cards/new/page.tsx`, `frontend/src/app/admin/layout.tsx`, `frontend/src/app/profile/page.tsx`, and `frontend/src/app/api/healthz/route.ts` (the last one for probe freshness, not auth, but the constant is the same).

The three `revalidate` states are documented in `docs/frontend.md` — `0` means no cache, `false` means cache forever, omitted means Next's default heuristic. Pick `0` for auth-sensitive; never collapse to a `number` default.

## `redirect()` from `next/navigation` only navigates from a sync render path — use `router.replace` inside async `.catch`

`next/navigation`'s `redirect()` works by throwing `NEXT_REDIRECT`, which Next's App Router intercepts during render or inside synchronous server-action / event-handler frames. Inside a `Promise.catch()` the throw is captured by the promise machinery and becomes the rejection of the chained promise — Next never sees it, the page does not navigate, and the only signal is the unhandled rejection in the console. The bug is silent: a session-expired branch handles its `UNAUTHENTICATED` correctly but leaves the user stuck on the failing page.

The fix in async / promise-chain contexts is `router.replace(target)` from `useRouter()` — it is fire-and-forget, schedules the navigation through the App Router, and does not depend on a thrown sentinel:

```tsx
fetchMore({ /* ... */ })
  .then(() => { /* success */ })
  .catch((err) => {
    const kind = classifyQueryError(err);
    if (kind?.kind === "unauthenticated") {
      // redirect() throws NEXT_REDIRECT to navigate. Inside a Promise's .catch()
      // that throw becomes the rejection of the chained promise (not a navigation),
      // so the page stays put and the only signal is the warn log. Use
      // router.replace(), which is fire-and-forget and schedules the navigation
      // through the App Router.
      router.replace("/");
      return;
    }
    // ...other branches
  });
```

**How to apply:** any handler that calls `redirect()` from inside a `.catch`, `.then`, `await`-after-promise, or `setTimeout` callback must convert to `router.replace(target)` (or `router.push(target)`). The reverse is also true: the synchronous-render and server-action paths (RSC body, route handler `GET`/`POST`, `useTransition` server-action callback) MUST keep `redirect()` because they have no `useRouter()` instance available. The two helpers are not interchangeable — pick by call-site context. Reference: `frontend/src/app/admin/users/AdminUsersClient.tsx` `handlePageChange` `.catch` (uses `router.replace("/")` for UNAUTHENTICATED inside a Promise chain).

## Surface non-blocking sibling-query failures via structured `console.warn` for operator triage

When a page fires a secondary "background" query whose failure does not block the primary flow (e.g. a faceted-filter dropdown loading its choices, a sidebar count, a prefetched chip's metadata), the natural failure shape is "render the empty state and move on". The empty state is then indistinguishable from "no data configured" — operators have no signal that the query is silently failing and the dropdown is permanently broken.

Observe the secondary `useQuery.error` field via a `useEffect` and emit a structured `console.warn` with the error's `name` only:

```ts
const rolesResult = useQuery(AdminRolesDocument, { fetchPolicy: "cache-first" });

// Surface AdminRoles query failure to operator triage. The dropdown silently
// degrades to empty (non-fatal for the rest of the page), but the failure must
// be observable in logs. err.message is omitted because backend GraphQL error
// messages may carry user-authored content (PII gate).
useEffect(() => {
  if (rolesResult.error) {
    console.warn("[admin-users] AdminRoles query failed", {
      name: rolesResult.error.name,
    });
  }
}, [rolesResult.error]);
```

**Why:** the page's primary failure branches (banner, redirect) are already covered by `classifyQueryError` on the main query. The secondary query has no UI affordance to fail loudly without harming the main flow — the warn is the only triage seam. Logging `err.name` only (not `err.message`) follows the same PII rule as transport-rejection logging: backend GraphQL error messages can echo user-authored content (cardgroup names, search terms, etc.) and must not enter the warn payload by default.

**How to apply:** any `useQuery` whose result is consumed for a non-blocking UI affordance (a filter dropdown, a count badge, a prefetch) must observe `result.error` in a sibling `useEffect` and emit a `[scope]` warn with `name` only. Reference: `frontend/src/app/admin/users/AdminUsersClient.tsx` (`AdminRoles query failed` warn for the role-filter Popover dropdown).

## Pages that bypass `AppShell` MUST render their own `<main>` landmark

The root layout in `frontend/src/app/layout.tsx` short-circuits `AppShell` for any route that owns the full viewport (today: `/login` via `pathname === "/login"`). When `AppShell` is bypassed, the rendered tree contains no `<main>`, no `<nav>`, and no shell-level landmarks — the page itself is the only place a landmark can be emitted. Without an explicit `<main>`, screen readers (VoiceOver, JAWS, NVDA) have no jump-to-content target and the page fails WCAG 2.1 SC 1.3.6 ("Identify Purpose").

```tsx
// frontend/src/app/login/page.tsx
return (
  <main data-testid="login-grid" className="relative grid h-svh lg:grid-cols-2">
    {/* page content */}
  </main>
);
```

**How to apply:** any page added to the `AppShell`-bypass branch of `app/layout.tsx` MUST render `<main>` as its outermost content wrapper. Co-locate a test that asserts `screen.getByRole("main")` resolves on the bypassed route — the assertion throws when the landmark is absent, so it is forcing rather than tautological. The shell-rendered routes do not need this rule because `AppShell` already emits the landmark at the layout level; a second `<main>` in a child page would create duplicate landmarks and confuse assistive technology.
