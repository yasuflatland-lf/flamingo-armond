# `router.replace` / `router.push` must propagate preserved query params explicitly

> Part of [`.claude/rules/frontend-typescript-conventions.md`](../frontend-typescript-conventions.md). See the index for related rules.

When code rewrites the URL via `router.replace` or `router.push` from inside a flow that already received user-controlled query params (e.g. `?return=`, `?next=`, `?welcome=1`), the rewrite must explicitly carry those params forward. A naive `router.replace(\`/cards/new?cardgroup=${id}\`)` from a picker handler drops every other param the user arrived with — including the `?return=` that was supposed to control post-create navigation. The user's intent is silently lost; there is no log, no redirect-to-default, no error. The next post-create step then routes the user somewhere they did not ask for.

```tsx
// AVOID: drops every other query param the user arrived with.
function handlePickerSelect(newId: string) {
  router.replace(`/cards/new?cardgroup=${encodeURIComponent(newId)}`, { scroll: false });
}

// PREFER: rebuild via URLSearchParams and re-attach already-sanitized params.
function handlePickerSelect(newId: string) {
  const params = new URLSearchParams({ cardgroup: newId });
  if (returnTo !== null) params.set("return", returnTo);  // already sanitized upstream
  router.replace(`/cards/new?${params.toString()}`, { scroll: false });
}
```

**Why:** the URL is the single source of truth for cross-component flow state in this codebase (see `docs/frontend.md` § "/cards/new cardgroup resolution"). A handler that rewrites part of the URL is implicitly responsible for preserving every other part — anything else silently breaks the contract that the URL drives the rendered tree. The fix is to use `URLSearchParams` to assemble the new query string from the **sanitized** values already in component scope (`returnTo` post-`sanitizeReturnTo`, never raw `searchParams.get(...)`); never re-read raw user-controlled strings from `searchParams` and concatenate them into the URL — that re-opens the open-redirect surface that `sanitizeReturnTo` was meant to close.

**How to apply:** any `router.replace` / `router.push` call inside a flow that has its own `?return=` / `?next=` / shared-flow-state query param must (a) build the new URL via `URLSearchParams`, not string interpolation, and (b) re-attach the param from a previously-sanitized variable, not from `searchParams.get(...)`. Pair the implementation with a regression test that mounts the component with a `return=` value and asserts both the happy path (param preserved) and the rejection path (open-redirect value not propagated). Reference: `frontend/src/app/cards/new/cards-new-client.tsx` `handlePickerSelect`.
