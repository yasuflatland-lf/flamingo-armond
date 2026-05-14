# Delayed-DELETE undo toast, SwipeableRow, and useReducedMotion

> Part of [`docs/frontend.md`](../frontend.md). See the index for related chapters.

See [`docs/frontend-undo-toast.md`](frontend-undo-toast.md) — sonner snackbar undo pattern, mobile swipe-to-delete row component, and the `useReducedMotion` JS hook. Split from this file because this file is at its 600-line soft cap.

The cleanest concrete failure mode: when header auth logic moved to `app/layout.tsx`, an old test suite was deleted, leaving critical branches untested. The suite had eight branches asserting that the root layout degrades silently on `getUser()` failure / `gqlFetch` `UNAUTHENTICATED` / `me`-fetch failure. The fix was `app/layout.test.tsx`, which re-asserts every branch against the post-refactor implementation. Reference: `frontend/src/app/layout.test.tsx`.
