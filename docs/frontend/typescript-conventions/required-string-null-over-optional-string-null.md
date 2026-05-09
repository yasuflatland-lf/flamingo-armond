# Required `string | null` over optional `?: string | null` for security-relevant or caller-deliberate props

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

`prop?: string | null` and `prop: string | null` are not interchangeable. The optional form (`?`) collapses three distinct caller states into two observable outcomes — callers may omit the prop entirely, which is indistinguishable at runtime from an explicit `null` and means the type system does not force the caller to acknowledge the prop's existence. When a prop has security implications (e.g. a redirect destination, a sanitized user-supplied value) or when the calling component must make an explicit choice (pass a value or acknowledge absence), use the required form:

```ts
// AVOID: callers can omit entirely; the prop's existence is unacknowledged.
interface Props { returnTo?: string | null; }

// PREFER: callers must pass something, even if it is null.
interface Props { returnTo: string | null; }
```

The required form surfaces callers that forgot to wire the prop (compile error: "returnTo is missing") rather than silently defaulting to `undefined`. Reference: `frontend/src/app/cardgroups/new/new-cardgroup-client.tsx` (`returnTo: string | null`) after a review finding that the optional form allowed callers to skip the prop and lose the sanitized redirect value without any error.

### Widen `string` to `string | null` rather than fabricating an empty-string default

When a value has natural absence semantics — e.g. a Supabase user without an email, an unset profile bio, an optional last-viewed cardgroup — type the field as `string | null` and let consumers branch on `null`. Falling back to `""` at the layout boundary (`email: user.email ?? ""`) collapses two distinct states into one observable outcome:

```tsx
// AVOID: "" loses the distinction between "no email on this account" and "email is the empty string".
<AppShell user={{ email: user.email ?? "" }} isAdmin={isAdmin} />

// PREFER: keep the absence-bearing type all the way to the consumer.
<AppShell user={{ email: user.email }} isAdmin={isAdmin} />

interface AppShellProps {
  user: { email: string | null } | null;
  // ...
}
```

The empty-string fallback is convenient because every consumer that does `user.email.length`, `user.email.toLowerCase()`, or `<span>{user.email}</span>` "just works" — but every one of those sites silently renders an empty string for the absence case, which is rarely the intended UI. Branch explicitly: `{user.email !== null && <span>{user.email}</span>}` mirrors the runtime invariant.

**Why:** `T | null` is a single forcing function; `T = ""` is a per-consumer convention that has to be re-asserted at every read site, and any consumer that forgets is a silent bug. The same rule extends to numeric fields where `0` is a legitimate value (use `number | null`, not `number = 0`) and to dates (use `Date | null`, not the epoch). Reference: `frontend/src/components/nav/global-rail.tsx` and `frontend/src/components/nav/app-shell.tsx` (`user: { email: string | null } | null`). This rule pairs with the required-vs-optional rule above: prefer `email: string | null` (required, nullable) over `email?: string` (optional, narrower-than-it-looks).
