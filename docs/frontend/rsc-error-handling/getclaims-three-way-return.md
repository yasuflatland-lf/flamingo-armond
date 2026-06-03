# `supabase.auth.getClaims()` has a three-way return — branch on `claimsData == null`

> Part of the [frontend RSC error handling](../../../.claude/rules/frontend-rsc-error-handling.md) rules.

The Supabase SSR client's `auth.getClaims()` return type is a union of three discriminated shapes, not two:

| Shape | `data` | `error` | Meaning |
|---|---|---|---|
| Success | `{ claims: {...} }` | `null` | Session valid, JWT parsed |
| Error | `null` | `AuthError` | JWT verification failed (key rotation, missing JWKS) |
| **TOCTOU race** | `null` | `null` | Session vanished between `getUser()` and `getClaims()` |

Code that branches only on `claimsError != null` silently degrades on the third path. The destructured `claimsData` is `null`, the subsequent `claimsData.claims.app_metadata?.role` access throws `Cannot read property 'claims' of null`, and the resulting error escapes the layout to `app/global-error.tsx` (or Next's default 500), even though the SDK reported no transport failure.

The race is real: a sign-out from another tab, a Supabase session revocation, or a cookie expiry between `await supabase.auth.getUser()` (which returns the cached user) and `await supabase.auth.getClaims()` (which re-reads the cookie and re-validates the JWT) all land in this window. It is rare in steady state but easy to reproduce on a stale-cookie cold render.

## Correct three-branch shape

```ts
const { data: claimsData, error: claimsError } = await supabase.auth.getClaims();
if (claimsError != null) {
  console.warn("[supabase/middleware] getClaims() failed — isAdmin defaulting to false:", {
    user_id: user.id,
    error_name: claimsError.name,
  });
} else if (claimsData == null) {
  console.warn("[supabase/middleware] getClaims() returned null data without error", {
    user_id: user.id,
  });
} else if (claimsData.claims == null) {
  console.warn("[supabase/middleware] getClaims() returned data without claims", {
    user_id: user.id,
  });
} else {
  isAdmin = claimsData.claims.app_metadata?.role === "admin";
}
```

Three reasons the warn-and-degrade form is preferred over a throw:

1. **Middleware cannot throw unhandled.** An uncaught exception in Next.js middleware produces a 500 for the entire request. The try/catch around `getClaims()` degrades to `isAdmin=false` so the app stays up. See [`docs/frontend/rsc-error-handling/structural-error-parsers-warn-on-shape-narrow.md`](structural-error-parsers-warn-on-shape-narrow.md) for the structural-parser sibling rule, and the Header degradation rule in [`.claude/rules/frontend-rsc-error-handling.md`](../../../.claude/rules/frontend-rsc-error-handling.md).
2. **`isAdmin` is a UI hint, not an enforcement boundary.** `app/admin/layout.tsx` is the actual gate; degrading the layout's `isAdmin` to `false` hides the admin rail entry but does not grant any access.
3. **The fourth shape (`{ data: { claims: null }, error: null }`) is forward-compatibility defence.** The SDK's current TypeScript types do not include this case, but an optional chain `.claims?.app_metadata` would silently degrade through it. An explicit `claimsData.claims == null` branch surfaces a future SDK type-drift via a warn rather than letting `isAdmin = false` happen invisibly.

## Discriminating-key payload + `toHaveBeenCalledTimes` on the test

The warn payload uses `user_id` (and only `user_id`) as the discriminator. `email` and `display_name` are PII and must not appear in the structured log payload — see [`redact-err-message-from-console-payloads.md`](redact-err-message-from-console-payloads.md) for the redaction rule. Tests assert both the shape and the call count:

```ts
expect(consoleWarnSpy).toHaveBeenCalledWith(
  "[supabase/middleware] getClaims() returned null data without error",
  expect.objectContaining({ user_id: userId }),
);
expect(consoleWarnSpy).toHaveBeenCalledTimes(1);
const warnPayload = consoleWarnSpy.mock.calls[0][1] as Record<string, unknown>;
expect(Object.keys(warnPayload)).not.toContain("email");
expect(Object.keys(warnPayload)).not.toContain("display_name");
```

The `toHaveBeenCalledTimes(1)` assertion is load-bearing: `mock.calls[0][1]` silently picks the first call without bounding the count, so a regression that emits a second warn (e.g. a refactor that double-fires the branch) would pass the PII assertion. See [`docs/frontend/typescript-conventions/tohavebeencalledtimes-before-mock-calls-access.md`](../typescript-conventions/tohavebeencalledtimes-before-mock-calls-access.md).

Reference: `frontend/src/lib/supabase/middleware.ts` (the canonical `getClaims()` call site — three-branch `{data,error}` shape plus outer try/catch for thrown non-AuthError exceptions), and the middleware test file for the three branches.

## `getClaims` can also THROW non-AuthError — wrap in try/catch outside an error boundary

`getClaims()` documents its return type as `{ data, error }`, but it can also **throw** outright — bypassing that shape entirely. Known throw sources as of `@supabase/auth-js@2.106.2`:

- A plain `Error` from the internal `validateExp` helper when the JWT is expired (`"JWT has expired"`) or lacks an `exp` claim (`"Missing exp claim"`).
- A `DOMException` from the WebCrypto `subtle.verify` call when the signature verification fails at the platform level.

These do not return `{ data: null, error: AuthError }`. They throw. The three-branch `{data,error}` handler above does not see them.

**Why this matters outside a React error boundary:** inside a React component tree, a thrown error propagates to the nearest `error.tsx` boundary and can be caught there. Outside a boundary — in Next.js middleware, a Route Handler, or a Vitest test — an uncaught throw produces a 500 for the entire request (middleware / Route Handler) or an unhandled-rejection test failure (Vitest). The fix is an outer `try/catch` that wraps the entire `getClaims()` call:

```ts
let isAdmin = false;
if (user) {
  try {
    const { data: claimsData, error: claimsError } = await supabase.auth.getClaims();
    // ... three-branch {data,error} handler ...
  } catch (err) {
    // getClaims() can throw non-AuthError exceptions (plain Error from validateExp,
    // DOMException from WebCrypto) that escape the SDK's internal AuthError catch.
    // isAdmin is a UI hint only — fail closed.
    console.warn(
      "[supabase/middleware] getClaims() threw unexpectedly — isAdmin defaulting to false:",
      err instanceof Error ? err.name : "unknown",
    );
  }
}
```

Log only `err.name` (never `err.message`) on the throw path — the PII rule from [`redact-err-message-from-console-payloads.md`](redact-err-message-from-console-payloads.md) applies: a JWT-related exception message may carry user-identifying content.
