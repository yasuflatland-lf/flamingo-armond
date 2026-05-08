import { useSyncExternalStore } from "react";

const MOBILE_BREAKPOINT = 768;

// Exported for direct unit testing of SSR / no-window branches that
// useSyncExternalStore inside jsdom never reaches.
export function subscribe(callback: () => void) {
  if (typeof window === "undefined") return () => {};
  const mql = window.matchMedia(`(max-width: ${MOBILE_BREAKPOINT - 1}px)`);
  mql.addEventListener("change", callback);
  return () => mql.removeEventListener("change", callback);
}

export function getSnapshot() {
  if (typeof window === "undefined") return false;
  return window.innerWidth < MOBILE_BREAKPOINT;
}

export function getServerSnapshot() {
  return false; // SSR default — matches the useReducedMotion convention in use-reduced-motion.ts
}

export function useIsMobile(): boolean {
  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
}
