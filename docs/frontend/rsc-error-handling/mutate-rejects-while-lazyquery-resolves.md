# `useMutation` rejects, `useLazyQuery` resolves — `result.error` after a mutation is dead code

> Part of the [frontend RSC error handling](../../../.claude/rules/frontend-rsc-error-handling.md) rules.

In `@apollo/client` v4 (the repo is on `^4.1.9`), the two hook execute functions report a GraphQL/network failure through **different channels** under the default `errorPolicy: "none"`:

- **`useMutation`'s mutate function REJECTS its promise.** After `const result = await runMutation(...)`, `result` is only ever the success value — `result.error` is never populated on a failure because control has already jumped to the `catch`. The error MUST be handled in a `try/catch`.
- **`useLazyQuery`'s execute function RESOLVES with `{ data, error }`.** A GraphQL/network failure surfaces as a populated `result.error`, not as a rejection. (Apollo still rejects the lazy-query promise on a thrown link error, so a `catch` is still the belt to the `result.error` braces — keep both.)

The consequence: an `if (result.error)` branch written after `await runMutation(...)` is **dead code that silently never fires**. The real handler is the `catch`. A reader who copies the lazy-query shape into a mutation handler ends up with an error branch that looks like coverage but never executes; the failure surfaces only as an unhandled rejection or a missing banner.

## Worked example

`frontend/src/components/batch-import/batch-import-wizard.tsx` exercises both shapes side by side:

- `handleValidate` calls `useLazyQuery`'s `runValidate` and correctly keeps **both** `if (result.error)` and a `catch` — the resolve path populates `result.error`, the reject path lands in `catch`.
- `handleImport` calls the injected `onImport` callback (which in the cardgroup wrapper `CardgroupBatchImportForm` calls `useMutation`'s `runImport` and reads `result.data?.importCards`) and relies on the `catch` **only** — there is no `if (result.error)` branch, because that branch could never run for a mutation. The success path treats a `null` return from `onImport` as its own banner case; everything else is a rejection caught by `catch`.

Both handlers route the caught/returned error through the shared `getBackendErrorBanner` helper (`@/lib/apollo/errors`), so the user-facing copy stays consistent regardless of which channel the error arrived on.

## Why this is not the same as the optimistic-rollback v3 note

`.claude/rules/pagination.md` notes that `@apollo/client` v3.x rolls back optimistic writes on network errors but not consistently on typed GraphQL errors. That is a **different** concern (optimistic-cache rollback timing) and is unchanged by this rule — do not conflate the two. This rule is about which channel (`reject` vs `resolve`) carries the error for a mutate-vs-lazy-query call, which determines whether a `result.error` branch is live or dead.

## Related rules

- [`isUnauthenticatedGraphQLError` matches gqlFetch — not Apollo Client runtime errors](apollo-runtime-vs-gqlfetch-error-shape.md) — once you are inside the `catch`, this covers extracting `extensions.code` from the runtime error shape (`CombinedGraphQLErrors.is(err)` + `err.errors`).
- [Pair every `try { ... } finally { setLoading(false) }` with a `catch` for transport rejections](pair-try-finally-with-catch-for-transport.md) — the mutation reject path is exactly the transport rejection this rule warns about.
