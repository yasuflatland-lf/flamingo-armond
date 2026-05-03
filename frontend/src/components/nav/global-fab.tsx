"use client";

import { Plus } from "lucide-react";
import { usePathname, useRouter } from "next/navigation";

// Hidden on /login (anonymous-only), /learn (full-bleed swipe UI),
// /admin (different audience), and /cards/new + /cardgroups/new (FAB target — would loop).
const HIDDEN_PATH_RE = /^\/(login|learn|admin|cards\/new|cardgroups\/new)(\/|$)/;

export function GlobalFAB() {
  const pathname = usePathname();
  const router = useRouter();

  if (HIDDEN_PATH_RE.test(pathname)) {
    return null;
  }

  return (
    <div className="md:hidden">
      <button
        type="button"
        aria-label="Add new card"
        onClick={() => router.push("/cards/new")}
        className="fixed bottom-[calc(env(safe-area-inset-bottom)+1rem)] right-4 z-50 flex h-14 w-14 items-center justify-center rounded-full bg-brand-primary text-brand-primary-foreground shadow-lg"
      >
        <Plus className="h-6 w-6" aria-hidden="true" />
      </button>
    </div>
  );
}
