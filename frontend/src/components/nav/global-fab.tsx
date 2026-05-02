"use client";

import { usePathname, useRouter } from "next/navigation";
import { Plus } from "lucide-react";

const HIDDEN_PATH_RE = /^\/(learn|admin|cards\/new|cardgroups\/new)(\/|$)/;

interface GlobalFABProps {
  lastViewedCardgroupId?: string | null;
}

export function GlobalFAB({ lastViewedCardgroupId }: GlobalFABProps) {
  const pathname = usePathname();
  const router = useRouter();

  if (HIDDEN_PATH_RE.test(pathname)) {
    return null;
  }

  const href = lastViewedCardgroupId
    ? `/cards/new?cardgroup=${lastViewedCardgroupId}`
    : "/cards/new";

  return (
    <button
      aria-label="Add new card"
      onClick={() => router.push(href)}
      className="fixed bottom-[calc(env(safe-area-inset-bottom)+1rem)] right-4 z-50 flex h-14 w-14 items-center justify-center rounded-full bg-brand-primary text-brand-primary-foreground shadow-lg"
    >
      <Plus className="h-6 w-6" aria-hidden="true" />
    </button>
  );
}
