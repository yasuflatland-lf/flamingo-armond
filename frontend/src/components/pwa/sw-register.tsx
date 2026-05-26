"use client";

import { useEffect } from "react";

// Registers /sw.js once after mount — production only.
//
// The cache-first static-asset rule in sw-template.js assumes /_next/static/
// URLs are content-hashed and immutable. That holds for `next build` but not
// for `next dev`: Turbopack reuses stable dev chunk URLs for HMR, so a service
// worker in dev serves a stale client chunk and triggers hydration mismatches.
// We therefore register only in production, and in dev actively unregister any
// worker left behind by a prior dev session so the cache self-heals.
//
// Guards, in order:
//   "serviceWorker" in navigator — skips unsupported browsers (both modes).
//   process.env.NODE_ENV          — dev/test unregisters instead of registering.
//   window.isSecureContext        — production register path only; skips plain
//                                   HTTP (avoids SecurityError).
// Failures are warned, never thrown.
export function SwRegister() {
  useEffect(() => {
    if (!("serviceWorker" in navigator)) return;

    if (process.env.NODE_ENV !== "production") {
      navigator.serviceWorker
        .getRegistrations()
        .then((registrations) => {
          for (const registration of registrations) registration.unregister();
        })
        .catch((err) => {
          console.warn("[pwa] service worker cleanup failed:", err);
        });
      return;
    }

    if (window.isSecureContext) {
      navigator.serviceWorker.register("/sw.js").catch((err) => {
        console.warn("[pwa] service worker registration failed:", err);
      });
    }
  }, []);

  return null;
}
