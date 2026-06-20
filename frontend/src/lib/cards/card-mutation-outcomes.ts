/**
 * Shared outcome types and search predicate for the card mutation hooks.
 *
 * The cards (`/cardgroups/[id]/cards`) and master-cards
 * (`/admin/masters/[id]/edit/cards`) screens both drive their create/update
 * mutations through `useEntityCardMutations`. Their typed outcome unions and
 * the front/back search predicate were byte-identical; centralizing them here
 * means a change lands once instead of being kept in sync by hand.
 */

/** Outcome of a create attempt; the client maps it to validation / sheet / banner state. */
export type CreateCardOutcome =
  | { status: "success" }
  | { status: "validation"; field: string; message: string }
  | { status: "unexpected" }
  | { status: "rejected" };

/** Outcome of an update attempt; the client maps it to the inline row validation error. */
export type UpdateCardOutcome =
  | { status: "success" }
  | { status: "validation"; field: string; message: string }
  | { status: "unexpected" }
  | { status: "rejected" };

/** Form values shared by create and update. */
export interface CardFormValues {
  front: string;
  back: string;
}

/**
 * True when the card's front or back contains the (trimmed, lowercased) search
 * term. An empty search matches everything. Decides whether a freshly-created
 * card should also be written into the active-filter cache entry.
 */
export function cardMatchesSearch(
  card: { front: string; back: string },
  searchValue: string,
): boolean {
  const normalized = searchValue.trim().toLowerCase();
  if (normalized === "") return true;
  return (
    card.front.toLowerCase().includes(normalized) || card.back.toLowerCase().includes(normalized)
  );
}
