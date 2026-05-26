# Schema non-null does not protect against partial-response null-bubble — guard at the consumer

> Part of the [frontend RSC error handling](../../../.claude/rules/frontend-rsc-error-handling.md) rules. Cross-references [`partial-response-gqlfetch.md`](partial-response-gqlfetch.md) for the transport-layer contract and [`.claude/rules/pagination.md`](../../../.claude/rules/pagination.md) for Connection-shape conventions.

A schema field declared non-null (e.g. `myCardgroupsConnection: CardgroupConnection!`) produces a non-optional TypeScript type in codegen — `data.myCardgroupsConnection` looks like a guaranteed value. The wire protocol disagrees: GraphQL over HTTP §5.2 allows a partial response where the resolver throws and `data.myCardgroupsConnection` arrives as `null`, with the failure surfacing only as an entry in `errors`. The TypeScript type from codegen and the runtime wire shape are decoupled — the type asserts what the schema *promises*, not what the transport *delivers*.

The silent-failure mode that motivates this rule:

```ts
// Looks safe because the codegen type says CardgroupConnection (non-null).
const edges = data.myCardgroupsConnection?.edges?.map((e) => e.node) ?? [];
// On a partial response, data.myCardgroupsConnection is null and edges is [].
// The user sees an empty list with no banner, no log, no retry signal.
```

The `?.` operator and the `?? []` fallback together turn a real authorization or backend failure into "you have zero cardgroups." The empty-list state is indistinguishable from the genuine empty-state case ([`empty-result-is-business-state-not-error.md`](empty-result-is-business-state-not-error.md)), so the on-call signal disappears too — both render the same `<AllCaughtUp />`-style UI.

## Guard pattern: RSC throws, client component warns and degrades

Consumers of a non-null Connection field must explicitly branch on the null case before destructuring. The branch is **asymmetric by component type**, mirroring [`frontend-rsc-error-handling.md`](../../../.claude/rules/frontend-rsc-error-handling.md):

```ts
// RSC (server component): throw — error boundary activates, logs surface.
if (data.myCardgroupsConnection == null) {
  throw new Error("myCardgroupsConnection missing from response");
}

// Client component: console.warn and render a degraded empty/banner state.
if (data?.myCardgroupsConnection == null) {
  console.warn("[scope] myCardgroupsConnection null on partial response");
  return <DegradedBanner />;
}
```

RSC throws because the error boundary at `app/<route>/error.tsx` can render a recoverable retry UI for the user, and the throw produces a real log entry the on-call engineer can find. Client components must not throw because there is no React error boundary above an in-flight Apollo callback — a throw inside `onCompleted` or a `useEffect` body crashes the subtree rather than degrading it. The `console.warn` line preserves the diagnostic signal; the degraded banner gives the user a path forward.

The guard pattern applies to every consumer of a non-null Connection field. Today's call sites for `myCardgroupsConnection`:

- `frontend/src/app/page.tsx` — HomePage RSC (throw branch).
- `frontend/src/app/cards/new/page.tsx` — cards/new RSC (throw branch).
- `frontend/src/app/cardgroups/page.tsx` — cardgroups RSC (throw branch).
- `frontend/src/components/cards/cardgroup-picker-sheet.tsx` — picker sheet client component (warn-and-degrade branch).

A new consumer of any non-null Connection field must pick a branch from the table above based on whether it is an RSC or a client component. The grep audit `grep -rn "Connection?\\.edges\\?\\.map\\|Connection\\.edges\\?\\.map" frontend/src/` should return zero results — the `?.` chaining is the smoking gun for an unguarded consumer.

## Why this is not a `gqlFetch` problem

[`partial-response-gqlfetch.md`](partial-response-gqlfetch.md) describes how `gqlFetch` handles auth-coded partial responses (re-throw) and non-auth partial responses (return data + warn). The non-auth branch is the one this rule covers: `gqlFetch` returned `data` with the failed field as `null` and emitted a `console.warn`, but the caller still has to inspect the field. There is no place inside `gqlFetch` to know which fields are required by the caller — that decision lives at the consumer site, which is why the guard belongs there.

For a **mutation** that has the same shape — null payload silently passing through `.then` instead of `.catch` — see [`fire-and-forget-mutation-warn-on-null-and-non-success.md`](fire-and-forget-mutation-warn-on-null-and-non-success.md). The write-path analogue requires a structured warn when `result.data?.<field>` is null, plus a separate warn when `__typename` does not match any known success variant, plus `liftGraphQLCodes(err)` in `.catch`.
