// Pure helper: maps a pathname to the FAB destination and accessible label.
const CARDGROUP_DETAIL_RE = /^\/cardgroups\/([^/]+)$/;
const CARDGROUP_CARDS_RE = /^\/cardgroups\/([^/]+)\/cards$/;
const CARDGROUP_EDIT_RE = /^\/cardgroups\/([^/]+)\/edit(\/|$)/;

export interface FabAction {
  href: string;
  label: string;
}

/**
 * Maps the current pathname to the FAB destination and accessible label.
 * Returns null when the FAB has nothing meaningful to show for this path
 * (caller should treat this as a hide signal).
 *
 * Pure function — no side effects, no imports from React or Next.js.
 */
export function resolveFabAction(pathname: string): FabAction | null {
  if (CARDGROUP_EDIT_RE.test(pathname)) {
    return null;
  }

  if (pathname === "/cardgroups") {
    return { href: "/cardgroups/new", label: "Add new cardgroup" };
  }

  const cardsMatch = CARDGROUP_CARDS_RE.exec(pathname);
  if (cardsMatch) {
    return { href: "/cards/new?cardgroup=" + cardsMatch[1], label: "Add new card" };
  }

  const detailMatch = CARDGROUP_DETAIL_RE.exec(pathname);
  if (detailMatch) {
    return { href: "/cards/new?cardgroup=" + detailMatch[1], label: "Add new card" };
  }

  return { href: "/cards/new", label: "Add new card" };
}
