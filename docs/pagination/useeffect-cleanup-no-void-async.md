# `useEffect` cleanup must not fire `void asyncFn()` for navigation-time side effects

> Part of the [pagination](../../.claude/rules/pagination.md) rules. Cross-referenced by `docs/backend.md` and `frontend/CLAUDE.md`.

Putting a flush call such as `flushPendingDeletes()` inside the `useEffect` cleanup function (`return () => { void asyncFn(); }`) is unreliable. The cleanup fires synchronously on unmount or dep-change; the returned Promise is voided; React does not keep the component alive while the async work runs; in-flight mutations can be abandoned mid-flight with no error surface.

**Why:** the cleanup callback is not an async boundary — `void asyncFn()` discards the Promise before the first `await` inside it can settle. Apollo's mutation dispatch happens inside the awaited network round-trip; voiding the Promise means the network request may never be enqueued, depending on how far execution got before the component was torn down.

**How to apply:** detect the pathname change via a `previousPathnameRef = useRef(pathname)` and fire the flush in the **effect body** when `pathname` differs from the ref. The async work runs while the component is still mounted and the Apollo client context is alive:

```ts
const previousPathnameRef = useRef(pathname);
useEffect(() => {
  if (previousPathnameRef.current !== pathname) {
    void flushPendingDeletes();
    previousPathnameRef.current = pathname;
  }
}, [pathname]);
```

This pairs with the [IntersectionObserver in-flight guard](intersection-observer-in-flight-guard.md): both patterns use a `useRef` to track external-trigger state that must not be read asynchronously. Reference: `frontend/src/app/cardgroups/[id]/cards/cards-client.tsx` `previousPathnameRef` pattern.
