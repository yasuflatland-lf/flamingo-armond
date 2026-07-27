"use client";

import { useSyncExternalStore } from "react";

/**
 * Reports offline only when navigator.onLine is false; true is unreliable on
 * captive portals and routeless LANs. The server snapshot stays false so server
 * and initial hydrated client renders agree.
 */
function subscribe(callback: () => void): () => void {
  if (typeof window === "undefined") return () => {};
  window.addEventListener("online", callback);
  window.addEventListener("offline", callback);
  return () => {
    window.removeEventListener("online", callback);
    window.removeEventListener("offline", callback);
  };
}

function getSnapshot(): boolean {
  if (typeof window === "undefined") return false;
  return window.navigator.onLine === false;
}

export function getServerSnapshot(): boolean {
  return false;
}

export function useIsOffline(): boolean {
  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
}
