# Fire-and-forget mutation: structured warn for null payload, non-success variant, and rejection

> Part of the [frontend RSC error handling](../../../.claude/rules/frontend-rsc-error-handling.md) rules.
> Closely related to [`partial-response-non-null-bubble-guard.md`](partial-response-non-null-bubble-guard.md)
> (the read-path analogue) and
> [`apollo-runtime-vs-gqlfetch-error-shape.md`](apollo-runtime-vs-gqlfetch-error-shape.md)
> (the `liftGraphQLCodes` helper used in the `.catch` branch).

A **fire-and-forget mutation** is dispatched via `client.mutate(...).then(...).catch(...)`
rather than a `useMutation` hook bound to JSX state. Two properties distinguish
it from a banner-driven mutation:

- The caller does not surface `loading` or `error` to the user — the mutation
  is a background effect (e.g. recording "last-viewed cardgroup" on mount).
- There is no automatic UI signal when the mutation fails. The user continues
  whatever flow triggered the dispatch.

The combination removes every UX-side observable failure mode. A silent
mutation that fails on every call produces zero user-facing symptoms; the
only signal the on-call engineer ever sees is a structured `console.warn`
or an error in the backend logs. **The warn at the call site is the
diagnostic contract.** Three failure shapes must each produce a distinct
warn, because each pinpoints a different root cause:

| Shape | Root cause | Warn signal |
|---|---|---|
| `.then(result => …)` with `result.data?.<field> == null` | Partial-response null bubble; auth or backend failure on the specific field | `console.warn("[scope] <op> returned null payload")` |
| `.then(result => …)` with `payload.__typename` not matching any known success variant | Typed `UserError` outcome (e.g. `InputValidationError`) the fire-and-forget caller cannot recover from, OR a new union variant the client was not regenerated against | `console.warn("[scope] <op> non-success variant", { typename })` |
| `.catch(err => …)` rejection | Transport failure or auth-code rejection (`UNAUTHENTICATED` / `FORBIDDEN`) | `console.warn("[scope] <op> failed", { name, codes: liftGraphQLCodes(err) })` |

Without all three warns, a silent partial-response null bubble or an
`UNAUTHENTICATED` mid-session rejection looks identical to the happy path
in the browser, and only the second-order consequence (e.g. the user lands
on the wrong page next time) reveals the regression.

## What

The pattern, from `frontend/src/app/learn/[cardgroupId]/learn-client.tsx`
where `setLastViewedCardgroup` is dispatched from a `useEffect`:

```ts
client
  .mutate({
    mutation: SetLastViewedCardgroupMutation,
    variables: { cardgroupId },
    update: (cache, { data }) => {
      // Narrow on __typename before the cache write so an InputValidationError
      // or unknown variant does not silently mutate the cache.
      const payload = data?.setLastViewedCardgroup;
      if (payload?.__typename !== "SetLastViewedCardgroupSuccess") return;
      // ... cache.writeFragment.
    },
  })
  .then((result) => {
    const payload = result.data?.setLastViewedCardgroup;
    if (!payload) {
      console.warn("[learn] setLastViewedCardgroup returned null payload");
      return;
    }
    if (payload.__typename !== "SetLastViewedCardgroupSuccess") {
      console.warn("[learn] setLastViewedCardgroup non-success variant", {
        typename: payload.__typename,
      });
    }
  })
  .catch((err) => {
    // err.message is omitted — backend messages may echo user-authored content.
    // See redact-err-message-from-console-payloads.md.
    console.warn("[learn] setLastViewedCardgroup failed", {
      cardgroupId,
      name: err instanceof Error ? err.name : "unknown",
      codes: liftGraphQLCodes(err),
    });
  });
```

Three properties make this pattern work:

1. **`update` callback gates the cache write on `__typename`.** A typed
   error variant (e.g. `InputValidationError`) carries no `user` field; an
   ungated `cache.writeFragment` would mutate the cache with `undefined`
   and silently corrupt every downstream query that reads the entity.

2. **`.then` branches on `!payload` before `__typename`.** The null-payload
   case (a partial-response null bubble per
   [`partial-response-gqlfetch.md`](partial-response-gqlfetch.md)) and the
   non-success-variant case are distinct root causes and deserve distinct
   warns. Conflating them into a single warn hides which failure mode
   surfaced.

3. **`.catch` includes `liftGraphQLCodes(err)` in the structured payload.**
   The codes array distinguishes `UNAUTHENTICATED` (the user's Supabase
   session was valid at dispatch time but invalidated by the time the
   request reached the backend) from `FORBIDDEN` (the user is signed in
   but cannot perform the action) from a transport failure (offline,
   timeout). The structured `liftGraphQLCodes` value is safe to log; the
   raw `err.message` is omitted per
   [`redact-err-message-from-console-payloads.md`](redact-err-message-from-console-payloads.md)
   because backend messages may echo user-authored content.

## How to apply

For every fire-and-forget mutation:

1. Inside the `update` callback, narrow on `data?.<field>?.__typename ===
   "<SuccessVariant>"` before touching the cache. Any other variant or a
   null payload must be a no-op for cache mutation.
2. In `.then`, branch on `!payload` first (warn and `return`), then on
   `payload.__typename !== "<SuccessVariant>"` (warn and continue — the
   user-visible flow proceeds; the diagnostic signal is the warn).
3. In `.catch`, log a structured payload with `name` (the error class) and
   `codes: liftGraphQLCodes(err)`. Never include `err.message`.
4. Each warn carries a `[scope]` prefix matching the source file's location
   (`[learn]`, `[cardgroups-new]`, etc.) so operator searches can pin
   down the originating call site without grepping the body string.

## Why not silently swallow the failure

A fire-and-forget mutation is "non-fatal" from the user's perspective but
not from the system's. The silent-failure mode that motivates this rule:

- `setLastViewedCardgroup` returns `InputValidationError` because the user's
  cardgroup was deleted in another tab and the dispatch races the cache
  invalidation. The mutation resolves successfully (`.then` runs, not
  `.catch`), the cache update is correctly skipped, the user keeps
  learning. On the next visit, the HomePage RSC reads
  `lastViewedCardgroup` from the server (which is now `null`), and the
  user lands on the cardgroup picker instead of resuming. There is no
  log entry tying the two events together unless the non-success warn
  fired at the time of the original dispatch.
- A partial-response null bubble with `UNAUTHENTICATED` reaches
  `gqlFetch`'s re-throw path (per
  [`partial-response-gqlfetch.md`](partial-response-gqlfetch.md))
  inside Apollo's transport, surfaces as a `CombinedGraphQLErrors`
  rejection, and lands in `.catch`. Without `liftGraphQLCodes`, the warn
  payload is indistinguishable from a transient network failure and the
  operator cannot tell whether to investigate auth or the backend.

References: `frontend/src/app/learn/[cardgroupId]/learn-client.tsx`
(`setLastViewedCardgroup` dispatch from `useEffect`).
