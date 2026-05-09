# `useSyncExternalStore` over `useState + useEffect` for browser-store subscriptions

> Part of [`.claude/rules/frontend-typescript-conventions.md`](../frontend-typescript-conventions.md). See the index for related rules.

A hook that subscribes to an external browser store (`matchMedia`, `localStorage`, `navigator.onLine`, `document.visibilityState`, `BroadcastChannel`, etc.) and surfaces its current value to React components has two common encodings: (a) `useState` seeded with a lazy initializer plus a `useEffect` that subscribes and re-`setState` on change, or (b) `useSyncExternalStore` with `subscribe` / `getSnapshot` / `getServerSnapshot` callbacks. React's docs explicitly list (b) as the right primitive for this case; the codebase enforces (b) for every browser-store hook.

```ts
// frontend/src/hooks/use-mobile.tsx
import { useSyncExternalStore } from "react";

const MOBILE_BREAKPOINT = 768;

function subscribe(callback: () => void) {
  if (typeof window === "undefined") return () => {};
  const mql = window.matchMedia(`(max-width: ${MOBILE_BREAKPOINT - 1}px)`);
  mql.addEventListener("change", callback);
  return () => mql.removeEventListener("change", callback);
}

function getSnapshot() {
  if (typeof window === "undefined") return false;
  return window.innerWidth < MOBILE_BREAKPOINT;
}

function getServerSnapshot() {
  return false; // SSR default
}

export function useIsMobile(): boolean {
  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
}
```

**Why:** the `useState + useEffect` shape carries three latent defects that are awkward to remove without rewriting the hook: (1) on first render the hook returns the initial state, then the effect fires post-mount and re-`setState` to the real value, producing a one-frame layout flash for any consumer whose render branches on the value; (2) consumers tend to defend against the flash by widening the type to `boolean | undefined` and collapsing the unset state with `!!isMobile` at the call site, which silently erases the "not yet measured" state; (3) the lazy initializer + effect resync is the canonical "duplicate state initialization" smell — the same value is computed once at mount and once again in the effect "in case it changed", which is evidence the wrong primitive was chosen. `useSyncExternalStore` returns the snapshot synchronously on first render and re-renders only when the subscribed callback fires, eliminating all three. Pair `subscribe` and `getSnapshot` with the `typeof window === "undefined"` guard so tests using `vitest`'s default `node` environment do not crash on import — the SSR snapshot path is what they exercise.

**How to apply:** any hook that observes a browser API and surfaces a primitive value to React MUST use `useSyncExternalStore`. Do not introduce a fresh `useState + useEffect` subscription pattern. The two standalone files in this codebase that follow the rule are `frontend/src/hooks/use-mobile.tsx` (matchMedia for layout breakpoint) and `frontend/src/lib/use-reduced-motion.ts` (matchMedia for `prefers-reduced-motion`); use either as the template. The `getServerSnapshot` value is part of the contract — pick a deterministic SSR default (`false` for "matches" predicates, `null` for absent values) and document it in a one-line comment. This rule supersedes any `useMounted` / `useIsMounted` pattern that wraps `useEffect(() => setMounted(true), [])`; see § "`next/dynamic({ ssr: false })` over a `useMounted` hook" below for the matching production-component pattern.
