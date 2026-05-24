import { safeDecodePathSegment } from "@/lib/safe-decode-path-segment";

const CARDGROUP_EDIT_RE = /^\/cardgroups\/([^/]+)\/edit(\/|$)/;
const LEARN_RE = /^\/learn\/([^/]+)(\/|$)/;

export type HeaderCreateAction =
  | { kind: "cardgroup"; label: "Add new cardgroup"; href: "/cardgroups/new" }
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
  | { kind: "role"; label: "Add new role" };

/**
 * Build a `card-with-group` action for the /cardgroups/:id/edit route.
 * `href` is URL-encoded, `cardgroupId` stays raw — see the JSDoc on the union
 * variant for the contract callers must observe.
 */
function cardWithGroup(rawId: string): Extract<HeaderCreateAction, { kind: "card-with-group" }> {
  const encodedId = encodeURIComponent(rawId);
  return {
    kind: "card-with-group",
    href: `/cards/new?cardgroup=${encodedId}`,
    label: "Add new card",
    cardgroupId: rawId,
  };
}

/**
 * Build a `card-with-group` action for the /learn/:id route.
 * Includes a `&return=/learn/<enc>` query param so the new-card flow can
 * redirect back to the learn screen after creation.
 * `href` is URL-encoded, `cardgroupId` stays raw.
 */
function cardWithGroupFromLearn(
  rawId: string,
): Extract<HeaderCreateAction, { kind: "card-with-group" }> {
  const encodedId = encodeURIComponent(rawId);
  return {
    kind: "card-with-group",
    href: `/cards/new?cardgroup=${encodedId}&return=/learn/${encodedId}`,
    label: "Add new card",
    cardgroupId: rawId,
  };
}

/**
 * Maps the current pathname to a "create" action descriptor for the nav header "+".
 *
 * Pure function — no side effects, no imports from React or Next.js.
 * Returns `null` when no create affordance applies to the given path.
 *
 * Routing rules (evaluated in order):
 * - `/cardgroups`          -> create new cardgroup
 * - `/cardgroups/:id/edit` -> create card pre-filled with the cardgroup
 * - `/learn/:id`           -> create card pre-filled with the cardgroup + return param
 * - `/admin/roles`         -> create new role
 * - anything else          -> `null`
 */
export function resolveHeaderCreateAction(pathname: string): HeaderCreateAction | null {
  if (pathname === "/cardgroups") {
    return { kind: "cardgroup", label: "Add new cardgroup", href: "/cardgroups/new" };
  }

  // Each match below has a required capture group 1, so `match[1] as string` is
  // sound (see docs/frontend/typescript-conventions/as-string-cast-on-regex-captures.md).
  const editMatch = CARDGROUP_EDIT_RE.exec(pathname);
  if (editMatch) {
    const rawId = safeDecodePathSegment(editMatch[1] as string);
    if (rawId === null) return null;
    return cardWithGroup(rawId);
  }

  const learnMatch = LEARN_RE.exec(pathname);
  if (learnMatch) {
    const rawId = safeDecodePathSegment(learnMatch[1] as string);
    if (rawId === null) return null;
    return cardWithGroupFromLearn(rawId);
  }

  if (pathname === "/admin/roles") {
    return { kind: "role", label: "Add new role" };
  }

  return null;
}
