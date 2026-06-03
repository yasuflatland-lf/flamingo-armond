# Skip auth-requiring GraphQL calls when the client knows the user is anonymous

> Part of the [frontend RSC error handling](../../../.claude/rules/frontend-rsc-error-handling.md) rules.

When an RSC has already determined the user is anonymous — either from `readAuthContext(await headers()).status` on standard pages, or from `supabase.auth.getUser()` on admin and login pages — do not issue any GraphQL query that requires `Authorization`. The backend will return `UNAUTHENTICATED`, the call site has to special-case the error, and the warn log fills with expected-and-uninteresting noise. Gate the call:

```ts
import { headers } from "next/headers";
import { readAuthContext } from "@/lib/supabase/auth-status";

// Standard page — reads the middleware-forwarded x-auth-status header
const { status } = readAuthContext(await headers());

let data = null;
if (status === "authenticated") {
  try {
    data = await gqlFetch(SomeAuthRequiredQuery, { revalidate: 0 });
  } catch (err) { /* ... */ }
}
```

This is the inverse of the "fail-closed in `gqlFetch`" rule documented in [`docs/frontend/profile-page-profile.md`](../profile-page-profile.md#authorization-forwarding-in-gqlfetch) § "Authorization forwarding in `gqlFetch`": `gqlFetch` itself throws on a session-fetch error rather than silently sending an anonymous request, but **callers** of `gqlFetch` are responsible for not invoking it in the first place when they already know the user is anonymous.
