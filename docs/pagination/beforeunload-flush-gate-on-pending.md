# `beforeunload` flush is browser-cancellable; gate the warn on pending count

> Part of the [pagination](../../.claude/rules/pagination.md) rules. Cross-referenced by `docs/backend.md` and `frontend/CLAUDE.md`.

A `beforeunload` listener that unconditionally fires `void flushPendingDeletes()` produces false-positive operator noise on every routine navigation even when there is nothing pending. More critically, modern browsers cancel pending `fetch` / XHR requests when `beforeunload` fires unless the request uses `keepalive: true` or `sendBeacon` — neither of which is viable for GraphQL endpoints that require `Authorization` headers. The flush is therefore a best-effort hint, not a guarantee.

**How to apply:** gate both the warn and the flush on `pendingRef.current.size > 0` to avoid signal noise on routine navigation. Document the browser-cancellation limitation in a code comment so a future maintainer does not replace the warn with a user-facing error message:

```ts
function onBeforeUnload() {
  if (pendingRef.current.size === 0) return;
  console.warn(
    "[UndoDeleteProvider] flushPendingDeletes on beforeunload — may be cancelled by browser",
  );
  void flushPendingDeletes();
}
```

**Why:** an unconditional warn desensitizes operators to the real signal — a non-zero pending count at unload time is worth triaging; a zero-pending unload is routine. The gate makes the warn a meaningful signal rather than noise. Reference: `frontend/src/lib/undo-delete.tsx` `onBeforeUnload` inside `UndoDeleteProvider`.
