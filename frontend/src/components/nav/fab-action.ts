const CARDGROUP_EDIT_RE = /^\/cardgroups\/([^/]+)\/edit(\/|$)/;
const LEARN_RE = /^\/learn\/([^/]+)$/;

export type FabAction =
  | { kind: "cardgroup"; href: "/cardgroups/new"; label: "Add new cardgroup" }
  | {
      kind: "card-with-group";
      /** Pre-encoded URL — already URL-safe, route via `router.push(href)` directly. */
      href: string;
      label: "Add new card";
      /**
       * Raw, unencoded cardgroup id (e.g. for display, analytics, or as a React key).
       * Do NOT interpolate into a URL without `encodeURIComponent` — `href` is the
       * correct field for navigation.
       */
      cardgroupId: string;
    }
  | { kind: "card"; href: "/cards/new"; label: "Add new card" };

/**
 * Build a `card-with-group` action from a raw cardgroup id captured from a
 * regex. `href` is URL-encoded, `cardgroupId` stays raw — see the JSDoc on
 * the union variant for the contract callers must observe.
 */
function cardWithGroup(
  rawId: string,
  options: { withReturnToLearn?: boolean } = {},
): Extract<FabAction, { kind: "card-with-group" }> {
  const encodedId = encodeURIComponent(rawId);
  const href = options.withReturnToLearn
    ? `/cards/new?cardgroup=${encodedId}&return=/learn/${encodedId}`
    : `/cards/new?cardgroup=${encodedId}`;
  return { kind: "card-with-group", href, label: "Add new card", cardgroupId: rawId };
}

/**
 * Maps the current pathname to the FAB destination and accessible label.
 * Returns null when the FAB has nothing meaningful to show for this path.
 * Each caller decides how to handle null (e.g. hide the FAB, or fall back to a default).
 *
 * Pure function — no side effects, no imports from React or Next.js.
 */
export function resolveFabAction(pathname: string): FabAction | null {
  if (pathname === "/cardgroups") {
    return { kind: "cardgroup", href: "/cardgroups/new", label: "Add new cardgroup" };
  }

  // Each match below has a required capture group 1, so `match[1] as string` is
  // sound (see frontend-typescript-conventions.md § "as string cast on regex captures").
  // /cardgroups/:id/edit is the integrated cardgroup management screen (cards
  // list + settings). /cardgroups/:id and /cardgroups/:id/cards both redirect
  // to /edit at the page level, so they are not handled here.
  const editMatch = CARDGROUP_EDIT_RE.exec(pathname);
  if (editMatch) return cardWithGroup(editMatch[1] as string);

  const learnMatch = LEARN_RE.exec(pathname);
  if (learnMatch) return cardWithGroup(learnMatch[1] as string, { withReturnToLearn: true });

  return { kind: "card", href: "/cards/new", label: "Add new card" };
}
