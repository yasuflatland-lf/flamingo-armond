"use client";

import { createContext, useContext, useEffect, useState } from "react";

type FabSuppressionValue = {
  suppressed: boolean;
  setSuppressed: (value: boolean) => void;
};

const FabSuppressionContext = createContext<FabSuppressionValue | null>(null);

/**
 * Lets a page suppress the layout-level {@link GlobalFAB} while it is mounted.
 * The FAB is rendered as a sibling of the route content in `app/layout.tsx`, so
 * a page that has no meaningful "add" action (e.g. the global `not-found.tsx`)
 * cannot hide it via the pathname-based guard — a 404 has an arbitrary path.
 * This context is the explicit opt-out seam instead.
 */
export function FabSuppressionProvider({ children }: { children: React.ReactNode }) {
  const [suppressed, setSuppressed] = useState(false);

  return (
    <FabSuppressionContext.Provider value={{ suppressed, setSuppressed }}>
      {children}
    </FabSuppressionContext.Provider>
  );
}

/**
 * Read whether the FAB is currently suppressed. Returns `false` outside a
 * provider so the FAB renders normally (and its unit tests need no wrapper).
 */
export function useFabSuppressed(): boolean {
  return useContext(FabSuppressionContext)?.suppressed ?? false;
}

/** Suppress the global FAB for as long as the calling component is mounted. */
export function useSuppressFab(): void {
  const ctx = useContext(FabSuppressionContext);
  useEffect(() => {
    if (!ctx) return;
    ctx.setSuppressed(true);
    return () => ctx.setSuppressed(false);
  }, [ctx]);
}
