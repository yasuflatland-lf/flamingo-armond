# Delayed-DELETE undo toast, SwipeableRow, and useReducedMotion

> Part of [`docs/frontend.md`](../frontend.md). See the index for related chapters.

See [`docs/frontend-undo-toast.md`](frontend-undo-toast.md) — sonner snackbar undo pattern, mobile swipe-to-delete row component, and the `useReducedMotion` JS hook. Split from this file because this file is at its 600-line soft cap.

The cleanest concrete failure mode: a deleted `global-header.test.tsx` had eight branches asserting that the root layout degrades silently on `getUser()` failure / `gqlFetch` `UNAUTHENTICATED` / `me`-fetch failure. Those branches now live in `app/layout.tsx`, but no test covered them after the deletion landed — the gap was caught only in review. The fix was a new `app/layout.test.tsx` that re-asserts every branch against the post-refactor implementation. Reference: `frontend/src/app/layout.test.tsx`.
