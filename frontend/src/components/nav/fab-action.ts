const CARDGROUP_DETAIL_RE = /^\/cardgroups\/([^/]+)$/;
const CARDGROUP_CARDS_RE = /^\/cardgroups\/([^/]+)\/cards$/;
const CARDGROUP_EDIT_RE = /^\/cardgroups\/([^/]+)\/edit(\/|$)/;
const LEARN_RE = /^\/learn\/([^/]+)$/;

export type FabAction =
  | { kind: "cardgroup"; href: "/cardgroups/new"; label: "Add new cardgroup" }
  | { kind: "card-with-group"; href: string; label: "Add new card"; cardgroupId: string }
  | { kind: "card"; href: "/cards/new"; label: "Add new card" };

/**
 * Maps the current pathname to the FAB destination and accessible label.
 * Returns null when the FAB has nothing meaningful to show for this path.
 * Each caller decides how to handle null (e.g. hide the FAB, or fall back to a default).
 *
 * Pure function — no side effects, no imports from React or Next.js.
 */
export function resolveFabAction(pathname: string): FabAction | null {
  if (CARDGROUP_EDIT_RE.test(pathname)) {
    return null;
  }

  if (pathname === "/cardgroups") {
    return { kind: "cardgroup", href: "/cardgroups/new", label: "Add new cardgroup" };
  }

  const cardsMatch = CARDGROUP_CARDS_RE.exec(pathname);
  if (cardsMatch) {
    // cardsMatch[1] is always defined when the regex matched (capture group 1 is required)
    const id = cardsMatch[1] as string;
    return {
      kind: "card-with-group",
      href: `/cards/new?cardgroup=${id}`,
      label: "Add new card",
      cardgroupId: id,
    };
  }

  const detailMatch = CARDGROUP_DETAIL_RE.exec(pathname);
  if (detailMatch) {
    // detailMatch[1] is always defined when the regex matched (capture group 1 is required)
    const id = detailMatch[1] as string;
    return {
      kind: "card-with-group",
      href: `/cards/new?cardgroup=${id}`,
      label: "Add new card",
      cardgroupId: id,
    };
  }

  const learnMatch = LEARN_RE.exec(pathname);
  if (learnMatch) {
    // learnMatch[1] is always defined when the regex matched (capture group 1 is required)
    const id = learnMatch[1] as string;
    return {
      kind: "card-with-group",
      href: `/cards/new?cardgroup=${id}&return=/learn/${id}`,
      label: "Add new card",
      cardgroupId: id,
    };
  }

  return { kind: "card", href: "/cards/new", label: "Add new card" };
}
