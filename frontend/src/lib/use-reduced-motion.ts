import { useEffect, useState } from "react";

/**
 * Returns true when the OS/browser requests reduced motion via the
 * `(prefers-reduced-motion: reduce)` media query, false otherwise.
 *
 * Updates reactively if the user changes the setting while the page is open.
 */
export function useReducedMotion(): boolean {
  const [reduced, setReduced] = useState<boolean>(() => {
    // Initialise from matchMedia on mount; default to false in SSR environments
    // where window is not defined.
    if (typeof window === "undefined") return false;
    return window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  });

  useEffect(() => {
    if (typeof window === "undefined") return;

    const mq = window.matchMedia("(prefers-reduced-motion: reduce)");

    const handler = (e: MediaQueryListEvent) => setReduced(e.matches);
    mq.addEventListener("change", handler);

    // Sync once on mount in case the media query state changed between
    // the lazy-init call and the effect running.
    setReduced(mq.matches);

    return () => mq.removeEventListener("change", handler);
  }, []);

  return reduced;
}
