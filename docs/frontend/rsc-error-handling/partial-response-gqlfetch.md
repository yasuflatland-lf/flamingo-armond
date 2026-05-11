# Partial-response errors in `gqlFetch`: auth codes re-throw, others return data + warn

> Part of the [`frontend RSC error handling`](../../../.claude/rules/frontend-rsc-error-handling.md) rules.

`gqlFetch` (`frontend/src/lib/apollo/server.ts`) handles a GraphQL response that carries both `data` and `errors` in two branches:

1. **Non-auth errors** — return `json.data` and emit a `console.warn` for debuggability. This follows GraphQL over HTTP §5.2: partial responses are valid, and callers should not silently lose data when some fields fail.

2. **Auth codes** (`UNAUTHENTICATED` or `FORBIDDEN`) — re-throw an error with the same `Error("GraphQL errors: " + JSON.stringify(json.errors))` format as the no-data path. This preserves the auth-redirect contract every RSC depends on — callers using `isUnauthenticatedGraphQLError` / `isForbiddenGraphQLError` continue to route through `redirect("/login")` or other auth gates correctly.

Without the re-throw, a partial response carrying `UNAUTHENTICATED` would silently bypass auth redirects. The scenario that motivates `hasAuthError` is a **non-null `data`** partial response where one field hit an auth failure but others returned data — for example a multi-field query:

```json
{ "data": { "publicConfig": { "featureFlags": [] } }, "errors": [{ "extensions": { "code": "UNAUTHENTICATED" }, "path": ["me"] }] }
```

If `gqlFetch` returned `json.data` here, the caller would receive the `publicConfig` field and never see the `UNAUTHENTICATED` signal on `me`. The re-throw ensures the caller's error boundary activates and can redirect to login.

Note: when `data` is `null`, the no-data throw branch fires first (line 79) and `hasAuthError` is never reached — the auth-code check is only relevant when `data != null`.

The internal `hasAuthError` helper (lines 22–28) checks for auth codes before deciding which branch to take:

```ts
function hasAuthError(errors: unknown): boolean {
  if (!Array.isArray(errors)) return false;
  return errors.some((e: { extensions?: { code?: unknown } }) => {
    const code = e?.extensions?.code;
    return code === "UNAUTHENTICATED" || code === "FORBIDDEN";
  });
}
```

This helper is intentionally not exported. The public surface for inspecting thrown errors is `isUnauthenticatedGraphQLError` and `isForbiddenGraphQLError` in `graphql-errors.ts` — these are transport-agnostic parsers that callers invoke after catching an error, whereas `hasAuthError` is a transport-layer decision inside `gqlFetch`.

The error message format **must match exactly** across both branches (no-data and partial-response-with-auth) so that `isUnauthenticatedGraphQLError` and `isForbiddenGraphQLError` consume both shapes identically. The literal prefix `"GraphQL errors: "` followed by `JSON.stringify(json.errors)` is the contract; any new code path that throws a GraphQL-errors error must use the same format.

When adding a new auth-related extension code, update both:
- The `hasAuthError` check in `server.ts`.
- The structural parsers in `graphql-errors.ts` (add a new exported helper like `isNewCodeGraphQLError`).

Substring matching the error message (e.g. `err.message.includes("UNAUTHENTICATED")`) is fragile and forbidden — see [`language-policy.md` § "Verify cross-doc symbols against the source of truth"](../../../.claude/rules/language-policy.md#verify-cross-doc-symbols-against-the-source-of-truth-never-against-a-sibling-doc).
