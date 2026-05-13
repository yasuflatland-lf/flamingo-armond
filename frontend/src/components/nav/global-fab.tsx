"use client";

import { Plus } from "lucide-react";
import { usePathname, useRouter } from "next/navigation";
import { resolveFabAction } from "./fab-action";

// Hidden on /login (anonymous-only), /admin (different audience),
// /cards/new + /cardgroups/new (FAB target — would loop),
// /profile (FAB action does not apply), and /learn (the LearnActionBar
// owns the primary interaction surface; no FAB needed there).
const HIDDEN_PATH_RE = /^\/(login|admin|cards\/new|cardgroups\/new|profile|learn)(\/|$)/;

export function GlobalFAB() {
  const pathname = usePathname();
  const router = useRouter();

  if (HIDDEN_PATH_RE.test(pathname)) {
    return null;
  }

  const action = resolveFabAction(pathname);
  if (action === null) {
    return null;
  }

  return (
    <div className="md:hidden">
      <button
        type="button"
        aria-label={action.label}
        onClick={() => router.push(action.href)}
        className="fixed bottom-[calc(env(safe-area-inset-bottom)+1rem)] right-4 z-50 flex h-14 w-14 items-center justify-center rounded-full bg-brand-primary text-brand-primary-foreground shadow-lg"
      >
        <Plus className="h-6 w-6" aria-hidden="true" />
      </button>
    </div>
  );
}
