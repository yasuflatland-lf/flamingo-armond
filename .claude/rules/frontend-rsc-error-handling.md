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

## Initial-load UNAUTHENTICATED redirects belong in the RSC `page.tsx`, not in the client `error.tsx`

A client `error.tsx` boundary that performs `router.replace("/login")` from a `useEffect` keyed on `error.message.includes("UNAUTHENTICATED")` is a band-aid: the boundary already received the error, the page rendered the boundary's fallback shell once, and the redirect fires after a post-render microtask. The user briefly sees the "Couldn't load your profile" copy before being navigated. More importantly, substring-matching `error.message` for routing decisions is the exact pattern § "Structurally parse GraphQL `extensions.code`" forbids — it conflates a real `UNAUTHENTICATED` extension with any error whose message text happens to contain the word.

The fix is to intercept the GraphQL error in the RSC `page.tsx` itself, before it bubbles into the boundary, using the structural helper:

```tsx
// frontend/src/app/profile/page.tsx
import { isUnauthenticatedGraphQLError } from "@/lib/apollo/graphql-errors";

export default async function ProfilePage() {
  const supabase = await createSupabaseServerClient();
  const { data: { user }, error: authErr } = await supabase.auth.getUser();
  if (authErr && authErr.name !== "AuthSessionMissingError") {
    console.error("[profile] getUser() failed:", authErr.name, authErr.message);
    throw authErr;
  }
  if (!user) redirect("/login");

  let data: MeQueryType;
  try {
    data = await gqlFetch(MeQuery, { revalidate: 0 });
  } catch (err) {
    if (isUnauthenticatedGraphQLError(err)) {
      redirect("/login");
    }
    console.error("[profile] gqlFetch failed:", err);
    throw err;
  }
  /* ...render with data... */
}
```

The `error.tsx` boundary then handles only the residual failure modes (network 5xx, infrastructure errors, mid-session UNAUTHENTICATED from client-side mutations after hydration). It logs the error with `digest` and renders a Retry button — no `router.replace`, no message-substring branching:

```tsx
// frontend/src/app/profile/error.tsx
"use client";

export default function ProfileError({ error, reset }: { error: Error & { digest?: string }; reset: () => void }) {
  useEffect(() => {
    // Initial-load UNAUTHENTICATED is intercepted in page.tsx and redirects to
    // /login before this boundary is reached. This boundary handles residual
    // failure modes (network, 5xx, post-hydration UNAUTHENTICATED).
    console.error("[/profile error boundary]", { message: error.message, digest: error.digest });
  }, [error]);
  return (/* ...heading + Retry button... */);
}
```

**Why:** the RSC has the GraphQL error in scope at the `try/catch` boundary BEFORE the client component ever renders, so the `redirect()` happens server-side and the user navigates without seeing the error fallback. Moving the redirect to the RSC also removes the substring-match dependency: `isUnauthenticatedGraphQLError` parses the structured extension, so the routing decision is correct even when the upstream message text changes. Pin the structural log assertion in the boundary's test — `expect.objectContaining({ message: "...", digest: "..." })` — using `digest` as the discriminating key (`Error.prototype` does not have `digest`) per `frontend-typescript-conventions.md` § "`expect.objectContaining({ message })` is not enough — add a discriminating key".

**How to apply:** any RSC `page.tsx` whose data fetch can return GraphQL `UNAUTHENTICATED` (i.e. any auth-gated query) MUST intercept the error and `redirect("/login")` from the page itself, using `isUnauthenticatedGraphQLError`. The sibling `error.tsx` keeps a small log-and-retry shell for residual failure modes; do not put `router.replace` in `error.tsx`. Today's pattern: `frontend/src/app/profile/page.tsx` (RSC redirect) + `frontend/src/app/profile/error.tsx` (degraded boundary). This rule pairs with § "Structurally parse GraphQL `extensions.code`" — both eliminate substring-matching on error.message at the routing seam.

## Mid-session UNAUTHENTICATED in a client component: degraded banner with `<Link href="/login">`, not `redirect()`

A client component (`"use client"`) that observes `UNAUTHENTICATED` from an Apollo `useQuery` cannot call `redirect()` from `next/navigation` — that helper is RSC-only and throws a runtime error inside a client tree. Calling `router.replace("/login")` is also wrong: the user may have unsaved local state (search input, half-typed form, scroll position) that vanishes on a hard navigation, and the redirect fires from a `useEffect` that produces the same brief flash described in § "Initial-load UNAUTHENTICATED redirects belong in the RSC".

The right shape is a **degraded banner** that surfaces the session-expired message inline and points at `/login` via a `<Link>` the user clicks when they are ready:

```tsx
// frontend/src/app/admin/users/AdminUsersClient.tsx
const queryErrorKind = classifyQueryError(queryError);

// UNAUTHENTICATED post-mount means the session expired while the page was
// open. The server-side gate in page.tsx + the admin layout already block
// the initial load (which redirects to "/"), so this only fires mid-session.
// Render a degraded banner pointing to /login rather than calling
// `redirect()` from a client component — see this rule.
{queryErrorKind?.kind === "unauthenticated" && (
  <div role="alert">
    <span>Your session has expired. </span>
    <Link href="/login" className="underline">Please sign in again.</Link>
  </div>
)}
```

The `classifyQueryError` helper (in `frontend/src/lib/apollo/errors.ts`) returns a typed `QueryErrorKind` discriminated union — `forbidden`, `unauthenticated`, or `banner` — so the JSX can branch on `kind` without re-implementing the structural extension parse at every call site.

**Why:** the initial-load case is already gated by the RSC `page.tsx` + the admin layout (§ above), so a mid-session UNAUTHENTICATED only fires when the access token expires while the page is alive. The user is already signed in to Supabase locally; a forced redirect throws away their in-page state and takes them to a login form they may not have asked for. The degraded-banner posture surfaces the session expiry, names the recovery, and lets the user choose when to navigate. This is the dual of the RSC rule: server-side initial load is "redirect immediately"; client-side mid-session is "render the banner."

**How to apply:** any client component that issues an auth-required query via Apollo MUST classify `queryError` via `classifyQueryError` and render a `<Link href="/login">` banner for the `unauthenticated` branch. Do not import `redirect` from `next/navigation` into a client component. Do not call `router.replace("/login")` from an effect keyed on the error. Reference: `frontend/src/app/admin/users/AdminUsersClient.tsx` (`queryErrorKind?.kind === "unauthenticated"` banner). The same posture applies to a `FORBIDDEN` mid-session case — render a permission-denied banner without a Retry button (re-issuing the query would fail again).

## Redact `err.message` from structured `console` payloads when the upstream may carry user content

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

**Why:** the substring-matching SDK error rule below (§ "Substring-matching SDK error strings") permits logging `err.message` on the **unmapped** path — but only when the SDK's error messages are server-generated and verifiably do not echo user input. GraphQL backends are the opposite: every typed-error constructor is free to inline the offending input into the message for clarity. The default posture for backend errors is therefore "redact `err.message`"; the substring-classifier carve-out applies only to SDKs whose contract guarantees server-only message provenance.

**How to apply:** every `console.warn` / `console.error` in a client component that catches a backend GraphQL error MUST omit `err.message` from the structured payload. Log `name` (typed via the `instanceof` widen) plus domain identifiers. The user-facing banner copy (`getBackendErrorBanner(err)` or `getBackendFieldErrors(err)?.<field>`) is what surfaces the error to the user; that path runs the parsed extension through a server-trusted classifier and is not the redaction concern. Pin the structural shape with `expect.objectContaining({ name: expect.any(String), cardgroupId: ... })` — using a domain-specific key (`cardgroupId`, `endCursor`) as the discriminator per `frontend-typescript-conventions.md` § "`expect.objectContaining({ message })` is not enough — add a discriminating key", and explicitly assert `expect(payload).not.toHaveProperty("message")` in at least one test per surface so a future contributor that adds `err.message` "for debugging" surfaces in CI. Reference: `frontend/src/app/learn/[cardgroupId]/learn-client.tsx` (handleSwipe + persist failures), `frontend/src/app/cardgroups/cardgroups-client.tsx` (fetchMore catch), `frontend/src/app/cardgroups/[id]/cards/cards-client.tsx` (fetchMore + bulk-delete catches), `frontend/src/app/admin/users/AdminUsersClient.tsx` (fetchMore catch).
