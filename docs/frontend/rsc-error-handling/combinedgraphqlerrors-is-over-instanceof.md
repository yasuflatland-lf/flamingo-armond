# Use `CombinedGraphQLErrors.is(err)` — never `instanceof CombinedGraphQLErrors`

> Part of the [frontend RSC error handling](../../../.claude/rules/frontend-rsc-error-handling.md) rules.

Apollo Client ships `CombinedGraphQLErrors.is(err)` as the canonical check because `instanceof` is unreliable across module realm boundaries. In a Next.js build, the server bundle and the client bundle each have their own copy of `@apollo/client/errors`; a `CombinedGraphQLErrors` created in one realm does not pass `instanceof` in the other. The `.is()` static method uses a duck-type check (`err?.graphQLErrors != null`) that survives the realm split.

Every helper in `frontend/src/lib/apollo/graphql-errors.ts` uses `.is()` already. Any new helper added to that file — or anywhere else that must detect a `CombinedGraphQLErrors` — must also use `.is()`:

```ts
// AVOID: fails silently when the error originates in a different bundle realm.
if (err instanceof CombinedGraphQLErrors) { ... }

// PREFER: duck-type check, realm-safe.
if (CombinedGraphQLErrors.is(err)) { ... }
```

**How to apply:** grep for `instanceof CombinedGraphQLErrors` before any merge; every hit is a bug. The lint rule does not catch this automatically — it must be verified in code review. Reference: `frontend/src/lib/apollo/errors.ts` (`getBackendFieldErrors`, `getBackendErrorBanner`, `classifyQueryError`) — all three use `CombinedGraphQLErrors.is(err)` as the canonical guard.
