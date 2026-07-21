# `supabase.auth.getClaims()` has a three-way return — branch on `claimsData == null`

> Part of the [frontend RSC error handling](../../../.claude/rules/frontend-rsc-error-handling.md) rules.

The Supabase SSR client's `auth.getClaims()` return type is a union of three discriminated shapes, not two:

| Shape | `data` | `error` | Meaning |
|---|---|---|---|
| Success | `{ claims: {...} }` | `null` | Session valid, JWT parsed |
| Error | `null` | `AuthError` | JWT verification failed (key rotation, missing JWKS) |
| **No session** | `null` | `null` | Anonymous request, or a session that vanished mid-request |

Code that branches only on `claimsError != null` silently degrades on the third path. The destructured `claimsData` is `null`, the subsequent `claimsData.claims.app_metadata?.role` access throws `Cannot read property 'claims' of null`, and in the middleware that uncaught throw becomes a 500 for the entire request (in a layout it would escape to `app/global-error.tsx`), even though the SDK reported no transport failure.

The third shape is not exotic: an anonymous visitor with no session cookie produces it on every request — `getClaims()` returns `{ data: null, error: null }` rather than an `AuthSessionMissingError`. A session lost mid-request lands in the same branch: a sign-out from another tab, a Supabase session revocation, or a cookie expiry between the middleware reading the request cookies and `await supabase.auth.getClaims()` re-validating the JWT. The anonymous case is the steady state; the mid-request loss is rare but easy to reproduce on a stale-cookie cold render.

## Correct three-branch shape

```ts
// frontend/src/lib/supabase/middleware.ts
let authStatus: AuthStatus = "anonymous";
let email = "";
let isAdmin = false;
const { data: claimsData, error: claimsError } = await supabase.auth.getClaims();
if (claimsError != null) {
  if (isStaleSessionError(claimsError)) {
    authStatus = "stale";
  } else if (isIgnorableAuthError(claimsError)) {
    authStatus = "anonymous";
  } else {
    authStatus = "error";
    // err.message omitted — a Supabase auth error message may carry user-identifying content.
    console.error("[supabase/middleware] getClaims() failed:", claimsError.name);
  }
} else if (claimsData == null) {
  // Anonymous request: no session. NOT an AuthSessionMissingError, and NOT a failure.
  authStatus = "anonymous";
} else {
  authStatus = "authenticated";
  email = claimsData.claims.email ?? "";
  isAdmin = claimsData.claims.app_metadata?.role === "admin";
}
```

**The `claimsData == null` branch must stay silent.** It is the steady state for every anonymous visitor, so a `console.warn` there fires on every unauthenticated request and floods operator logs with exactly the expected-and-uninteresting noise that [`skip-auth-graphql-calls-when-anonymous.md`](skip-auth-graphql-calls-when-anonymous.md) exists to prevent. Reclassify and move on; log nothing.

Three reasons the classify-and-degrade form is preferred over a throw:

1. **Middleware cannot throw unhandled.** An uncaught exception in Next.js middleware produces a 500 for the entire request. The try/catch around `getClaims()` degrades to `authStatus = "error"` with `isAdmin` false so the app stays up. See [`docs/frontend/rsc-error-handling/structural-error-parsers-warn-on-shape-narrow.md`](structural-error-parsers-warn-on-shape-narrow.md) for the structural-parser sibling rule, and the Header degradation rule in [`.claude/rules/frontend-rsc-error-handling.md`](../../../.claude/rules/frontend-rsc-error-handling.md).
2. **`isAdmin` is a UI hint, not an enforcement boundary.** `app/admin/layout.tsx` is the actual gate; degrading the layout's `isAdmin` to `false` hides the admin rail entry but does not grant any access.
3. **Only the genuinely unexpected error branch logs.** `isStaleSessionError` and `isIgnorableAuthError` (`frontend/src/lib/supabase/auth-errors.ts`) name the expected failures — a revoked session, a deleted user, a request with no session at all — and reclassify them into `stale` / `anonymous` without a log. Reserving `console.error` for the residual case keeps the log signal-bearing.

A note on the shape the code does **not** branch on: the SDK types `claims` as non-null on the success shape, so the middleware reads `claimsData.claims` directly and there is no `claimsData.claims == null` branch. If a future SDK version widens the type, add an explicit fourth branch rather than reaching for an optional chain — `claimsData.claims?.app_metadata` would degrade `isAdmin` to `false` invisibly, whereas an explicit branch surfaces the type drift.

## Log the error name only — there is no payload to discriminate

Both auth logs pass a bare string as the second argument, not a structured object:

- Error branch: `console.error("[supabase/middleware] getClaims() failed:", claimsError.name)`.
- Throw path: `console.warn("[supabase/middleware] getClaims() threw unexpectedly — degrading to error status:", err instanceof Error ? err.name : "unknown")`.

There is deliberately no `user_id` or `email` key. When `getClaims()` fails there is no verified `sub` claim to log, and the request's email is exactly the PII the redaction rule forbids — see [`redact-err-message-from-console-payloads.md`](redact-err-message-from-console-payloads.md). Assert the prefix, the name, and the call count:

```ts
expect(consoleErrorSpy).toHaveBeenCalledTimes(1);
expect(consoleErrorSpy).toHaveBeenCalledWith(
  "[supabase/middleware] getClaims() failed:",
  "AuthApiError",
);
```

`toHaveBeenCalledTimes` is load-bearing twice over here. It bounds the count so a second emission cannot hide behind the first, and it is the only assertion that catches a regression which starts logging on the silent `claimsData == null` branch.

Where a log site does carry a structured payload — `console.warn("[learn] setLastViewedCardgroup failed", { cardgroupId, name })` in `frontend/src/app/learn/[cardgroupId]/learn-client.tsx` — pin it with a domain-specific discriminating key and bound the count before indexing `mock.calls[0]`, because `mock.calls[0][1]` silently picks the first call. See [`docs/frontend/typescript-conventions/tohavebeencalledtimes-before-mock-calls-access.md`](../typescript-conventions/tohavebeencalledtimes-before-mock-calls-access.md).

Reference: `frontend/src/lib/supabase/middleware.ts` (the canonical `getClaims()` call site — three-branch `{data,error}` shape plus outer try/catch for thrown non-AuthError exceptions), and the middleware test file for the three branches.

## `getClaims` can also THROW non-AuthError — wrap in try/catch outside an error boundary

`getClaims()` documents its return type as `{ data, error }`, but it can also **throw** outright — bypassing that shape entirely. Known throw sources as of `@supabase/auth-js@2.106.2`:

- A plain `Error` from the internal `validateExp` helper when the JWT is expired (`"JWT has expired"`) or lacks an `exp` claim (`"Missing exp claim"`).
- A `DOMException` from the WebCrypto `subtle.verify` call when the signature verification fails at the platform level.

These do not return `{ data: null, error: AuthError }`. They throw. The three-branch `{data,error}` handler above does not see them.

**Why this matters outside a React error boundary:** inside a React component tree, a thrown error propagates to the nearest `error.tsx` boundary and can be caught there. Outside a boundary — in Next.js middleware, a Route Handler, or a Vitest test — an uncaught throw produces a 500 for the entire request (middleware / Route Handler) or an unhandled-rejection test failure (Vitest). The fix is an outer `try/catch` that wraps the entire `getClaims()` call:

```ts
let authStatus: AuthStatus = "anonymous";
try {
  const { data: claimsData, error: claimsError } = await supabase.auth.getClaims();
  // ... three-branch {data,error} handler ...
} catch (err) {
  // getClaims() can throw non-AuthError exceptions (plain Error from validateExp,
  // DOMException from WebCrypto) that escape the SDK's internal AuthError catch.
  // Fail closed to the degraded (logo-only) shell — an uncaught throw would 500
  // the whole request.
  authStatus = "error";
  console.warn(
    "[supabase/middleware] getClaims() threw unexpectedly — degrading to error status:",
    err instanceof Error ? err.name : "unknown",
  );
}
```

Log only `err.name` (never `err.message`) on the throw path — the PII rule from [`redact-err-message-from-console-payloads.md`](redact-err-message-from-console-payloads.md) applies: a JWT-related exception message may carry user-identifying content.
