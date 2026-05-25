"use client";

import { useEffect } from "react";

// Registers /sw.js once after mount.
// Guards: "serviceWorker" in navigator — skips unsupported browsers.
//         window.isSecureContext       — skips plain HTTP (avoids SecurityError).
// Failures are warned, never thrown.
export function SwRegister() {
  useEffect(() => {
    if ("serviceWorker" in navigator && window.isSecureContext) {
      navigator.serviceWorker.register("/sw.js").catch((err) => {
        console.warn("[pwa] service worker registration failed:", err);
      });
    }
  }, []);

  return null;
}
