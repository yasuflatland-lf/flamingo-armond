# Skip auth-requiring GraphQL calls when the client knows the user is anonymous

> Part of the [frontend RSC error handling](../../../.claude/rules/frontend-rsc-error-handling.md) rules.

When an RSC has already determined `user == null` from `supabase.auth.getUser()`, do not issue any GraphQL query that requires `Authorization`. The backend will return `UNAUTHENTICATED`, the call site has to special-case the error, and the warn log fills with expected-and-uninteresting noise. Gate the call:

```ts
let data = null;
if (user) {
  try {
    data = await gqlFetch(SomeAuthRequiredQuery, { revalidate: 0 });
  } catch (err) { /* ... */ }
}
```

This is the inverse of the "fail-closed in `gqlFetch`" rule documented in `frontend/CLAUDE.md` § "Authorization forwarding in `gqlFetch`": `gqlFetch` itself throws on a session-fetch error rather than silently sending an anonymous request, but **callers** of `gqlFetch` are responsible for not invoking it in the first place when they already know the user is anonymous.
