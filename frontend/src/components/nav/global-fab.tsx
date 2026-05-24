"use client";

import { Plus } from "lucide-react";
import { usePathname, useRouter } from "next/navigation";
import { resolveFabAction } from "./fab-action";
import { useFabSuppressed } from "./fab-suppression";

// Hidden on /login (anonymous-only), /admin (different audience),
// /cards/new + /cardgroups/new (FAB target — would loop),
// /profile (FAB action does not apply), and /learn (the LearnActionBar
// owns the primary interaction surface; no FAB needed there).
const HIDDEN_PATH_RE = /^\/(login|admin|cards\/new|cardgroups\/new|profile|learn)(\/|$)/;

export function GlobalFAB() {
  const pathname = usePathname();
  const router = useRouter();
  // A page can opt out of the FAB while mounted (e.g. not-found.tsx, which has
  // an arbitrary path that the pathname guard below cannot match).
  const suppressed = useFabSuppressed();

  // "/" is a server-redirect-only hub — HomePage always redirects (to /learn,
  // /cardgroups, /login, or /onboarding) and never renders content. Rendering
  // the FAB there only produces a flash during transitions that pass through
  // it (e.g. tapping the logo from /learn routes /learn -> "/" -> /learn).
  if (suppressed || pathname === "/" || HIDDEN_PATH_RE.test(pathname)) {
    return null;
  }

  const action = resolveFabAction(pathname);
  if (action === null) {
    return null;
  }
  const resolvedAction = action;

  function handleClick() {
    if (resolvedAction.kind === "card-with-group") {
      const event = new CustomEvent("flamingo:add-card", {
        cancelable: true,
        detail: { cardgroupId: resolvedAction.cardgroupId },
      });
      if (window.dispatchEvent(event)) {
        router.push(resolvedAction.href);
      }
      return;
    }

    if (resolvedAction.kind === "cardgroup") {
      // CardgroupsClient (mounted on /cardgroups) cancels this to open the
      // create drawer in place; when unhandled we fall back to the full page.
      const event = new CustomEvent("flamingo:add-cardgroup", { cancelable: true });
      if (window.dispatchEvent(event)) {
        router.push(resolvedAction.href);
      }
      return;
    }

    router.push(resolvedAction.href);
  }

  return (
    <div className="md:hidden">
      <button
        type="button"
        aria-label={resolvedAction.label}
        onClick={handleClick}
        className="fixed bottom-[calc(env(safe-area-inset-bottom)+1rem)] right-4 z-50 flex h-14 w-14 items-center justify-center rounded-full bg-brand-primary text-brand-primary-foreground shadow-lg"
      >
        <Plus className="h-6 w-6" aria-hidden="true" />
      </button>
    </div>
  );
}
