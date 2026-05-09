# Module-level singleton `Map` keyed by entity id requires a non-empty-string guard at the entry point

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

A module-level `Map<string, T>` (e.g. a pending-timer registry keyed by entity id) gives `""` an equal claim to be a valid key as any UUID. Two callers that independently pass `""` — a "stub id" code path, a short-circuit branch, a future caller that skips id resolution — collide silently: the second call's `cancelPending("")` cancels the first's timer, the first entry's `commitDelete` is never invoked, the cache stays in the optimistically-removed state, and the server never receives the DELETE.

The fix is a synchronous throw at the top of the public entry point. This is a programming-error guard, not user-input validation — the same level of force as the constructor-panic pattern in `.claude/rules/go-library-gotchas.md` § "Constructor panics are the right tool for non-empty config requires non-nil deps". The throw's stack trace names the bad call site; a `try/catch` that swallows it is a separate review concern at the swallowing site, not this module's problem.

```ts
export function scheduleDelete(opts: ScheduleDeleteOptions): ScheduleDeleteHandle {
  if (!opts.id) {
    throw new Error("undo-delete: id must be a non-empty string");
  }
  // ...
}
```

**Why:** `Map.get("")` and `Map.get(someRealId)` are both `O(1)` lookups. There is no runtime signal when two unrelated call sites share `""` as a key — no duplicate-key warning, no assertion, no type error. The only gate available is an explicit guard at the entry point.

**How to apply:** any module that exposes a public function accepting an id that keys into a module-level `Map` must validate `!id` (or `id === ""`) at the top of that function and throw synchronously. Pair with a co-located test that asserts the throw is synchronous and the message matches verbatim. Reference: `frontend/src/lib/undo-delete.ts` `scheduleDelete`.
