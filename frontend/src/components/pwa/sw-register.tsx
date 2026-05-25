"use client";

import { useEffect } from "react";

// Registers /sw.js as a service worker once after the component mounts.
// Guards with three conditions before attempting registration:
//   1. typeof window !== "undefined"  — belt-and-suspenders guard against the
//      code running outside a browser environment (e.g. a non-standard runtime).
//      This check is NOT an SSR guard: useEffect callbacks never run during
//      server-side rendering, so SSR is not a concern here.
//   2. "serviceWorker" in navigator   — skips on browsers that don't support SW.
//   3. window.isSecureContext         — skips on plain HTTP (SW requires HTTPS
//      or localhost); avoids a SecurityError that would otherwise surface as
//      an unhandled rejection in development over plain HTTP tunnels.
// Failures are caught and logged as warnings — registration failure must never
// break page rendering or app functionality.
export function SwRegister() {
  useEffect(() => {
    if (typeof window !== "undefined" && "serviceWorker" in navigator && window.isSecureContext) {
      navigator.serviceWorker.register("/sw.js").catch((err) => {
        console.warn("[pwa] service worker registration failed:", err);
      });
    }
  }, []);

  return null;
}
