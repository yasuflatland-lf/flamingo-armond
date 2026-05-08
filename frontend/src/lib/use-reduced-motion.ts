import { useSyncExternalStore } from "react";

/**
 * Returns true when the OS/browser requests reduced motion via the
 * `(prefers-reduced-motion: reduce)` media query, false otherwise.
 *
 * Updates reactively if the user changes the setting while the page is open.
 * Uses useSyncExternalStore to avoid the duplicate state-initialization that
 * a useState + useEffect approach produces (lazy initializer fires on the
 * render pass, then setReduced fires again in the effect after mount).
 */

const REDUCED_MOTION_QUERY = "(prefers-reduced-motion: reduce)";

// Exported for direct unit testing of SSR / no-window branches that
// useSyncExternalStore inside jsdom never reaches.
export function subscribe(callback: () => void): () => void {
  if (typeof window === "undefined") return () => {};
  const mq = window.matchMedia(REDUCED_MOTION_QUERY);
  mq.addEventListener("change", callback);
  return () => mq.removeEventListener("change", callback);
}

export function getSnapshot(): boolean {
  if (typeof window === "undefined") return false;
  return window.matchMedia(REDUCED_MOTION_QUERY).matches;
}

export function getServerSnapshot(): boolean {
  return false;
}

export function useReducedMotion(): boolean {
  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
}
