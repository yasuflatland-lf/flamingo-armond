"use client";

import { useEffect } from "react";
import { BrandSplash } from "@/components/pwa/brand-splash";

type RootErrorProps = {
  error: Error & { digest?: string };
  reset: () => void;
};

/**
 * Root error boundary for the page subtree.
 *
 * Catches errors thrown by any route segment without its own `error.tsx` — most
 * importantly the root redirect's failed backend fetch when the backend is
 * stopped or unreachable. It renders the same branded coral splash as the
 * loading state (static mark) plus a retry control, so a backend outage degrades
 * to a clean, on-brand screen rather than a blank/error page. `reset()` re-runs
 * the failed segment, which re-enters the loading splash while the fetch retries.
 *
 * Errors thrown from the root layout itself are still handled by
 * `global-error.tsx`.
 */
export default function RootError({ error, reset }: RootErrorProps) {
  useEffect(() => {
    // Scope prefix for log streams. Log the error name + digest only, never the
    // message (which may carry user-supplied content) — mirrors the middleware
    // and global-error boundaries.
    console.error("[error]", { name: error.name, digest: error.digest });
  }, [error]);

  return (
    <BrandSplash spin={false}>
      <div className="space-y-2">
        <h1 className="text-xl font-semibold tracking-tight">We couldn't load the app</h1>
        <p className="mx-auto max-w-xs text-sm text-white/90">
          This can happen when the server is starting up. Please try again in a moment.
        </p>
      </div>
      <button
        type="button"
        onClick={reset}
        className="rounded-lg bg-white px-8 py-3 text-sm font-medium text-brand-primary transition-colors hover:bg-white/90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white focus-visible:ring-offset-2 focus-visible:ring-offset-brand-primary"
      >
        Try again
      </button>
    </BrandSplash>
  );
}
