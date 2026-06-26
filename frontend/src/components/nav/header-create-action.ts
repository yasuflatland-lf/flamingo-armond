import { safeDecodePathSegment } from "@/lib/safe-decode-path-segment";

const CARDGROUP_EDIT_RE = /^\/cardgroups\/([^/]+)\/edit(\/|$)/;
const LEARN_RE = /^\/learn\/([^/]+)(\/|$)/;
const MASTER_EDIT_RE = /^\/admin\/masters\/([^/]+)\/edit(\/|$)/;

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
  // No href: role creation has no separate-page target — it opens the create
  // sheet by appending ?new=true to the current path (via useSheetSearchParam).
  | { kind: "role"; label: "Add new role" }
  // No href: master creation opens the create sheet the same way as role —
  // ?new=true on the current path (via useSheetSearchParam).
  | { kind: "master"; label: "Add new master" }
  // Deck-detail edit screens turn the "+" into an Add menu (Add card / Batch
  // import / Merge). `addCardHref` is the pre-encoded /cards/new fallback for the
  // cardgroup add-card item (master has no separate-page fallback).
  | {
      kind: "deck-add-menu";
      deck:
        | { kind: "master"; masterId: string }
        | { kind: "cardgroup"; cardgroupId: string; addCardHref: string };
    };

/** Narrowed type for the `deck-add-menu` union member. */
export type DeckAddMenu = Extract<HeaderCreateAction, { kind: "deck-add-menu" }>;

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
 * - `/cardgroups/:id/edit` -> deck-add-menu (cardgroup variant)
 * - `/learn/:id`           -> create card pre-filled with the cardgroup + return param
 * - `/admin/roles`            -> create new role
 * - `/admin/masters`          -> create new master
 * - `/admin/masters/:id/edit` -> deck-add-menu (master variant)
 * - anything else             -> `null`
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
    return {
      kind: "deck-add-menu",
      deck: {
        kind: "cardgroup",
        cardgroupId: rawId,
        addCardHref: `/cards/new?cardgroup=${encodeURIComponent(rawId)}`,
      },
    };
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

  if (pathname === "/admin/masters") {
    return { kind: "master", label: "Add new master" };
  }

  const masterEditMatch = MASTER_EDIT_RE.exec(pathname);
  if (masterEditMatch) {
    const rawId = safeDecodePathSegment(masterEditMatch[1] as string);
    if (rawId === null) return null;
    return { kind: "deck-add-menu", deck: { kind: "master", masterId: rawId } };
  }

  return null;
}
