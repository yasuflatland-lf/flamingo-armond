// Typed contract for the global-shell <-> feature-page event bus.
//
// The nav header (LogoDrawer) and feature pages communicate through a handful
// of `window` CustomEvents: the header dispatches and a page claims (the "+"
// create affordance, the search magnifier), or a page dispatches and the header
// reacts (the search-state active dot / aria-expanded). Centralizing the names
// and detail shapes here makes the protocol compiler-enforced — a typo'd event
// name or detail field becomes a type error rather than a silent runtime no-op.

/** `flamingo:search-state` payload: page -> header (active dot + aria-expanded). */
export interface SearchStateDetail {
  /** A non-empty filter is applied. */
  active: boolean;
  /** The takeover bar is open. */
  visible: boolean;
}

/** `flamingo:add-card` payload: header "+" -> the in-context card sheet. */
export interface AddCardDetail {
  cardgroupId: string;
}

/** `flamingo:add-master-card` payload: header "+" -> the master-cards sheet. */
export interface AddMasterCardDetail {
  masterId: string;
}

// Augment WindowEventMap so add/removeEventListener infer the CustomEvent detail
// type at every call site with zero per-site annotation. Keys must be string
// literals, so they are spelled out here and mirrored by FLAMINGO_EVENT below.
declare global {
  interface WindowEventMap {
    "flamingo:open-search": CustomEvent<undefined>;
    "flamingo:search-state": CustomEvent<SearchStateDetail>;
    "flamingo:add-cardgroup": CustomEvent<undefined>;
    "flamingo:add-card": CustomEvent<AddCardDetail>;
    "flamingo:add-master-card": CustomEvent<AddMasterCardDetail>;
  }
}

/** Canonical event names — reference these instead of raw string literals. */
export const FLAMINGO_EVENT = {
  openSearch: "flamingo:open-search",
  searchState: "flamingo:search-state",
  addCardgroup: "flamingo:add-cardgroup",
  addCard: "flamingo:add-card",
  addMasterCard: "flamingo:add-master-card",
} as const;

type FlamingoEventName = (typeof FLAMINGO_EVENT)[keyof typeof FLAMINGO_EVENT];

type FlamingoDetail<K extends FlamingoEventName> =
  WindowEventMap[K] extends CustomEvent<infer D> ? D : never;

/**
 * Type-safe `window.dispatchEvent(new CustomEvent(...))`. Events whose detail is
 * `undefined` (open-search, add-cardgroup) take no init; events with a detail
 * require it. Returns the `dispatchEvent` boolean — `false` means a listener
 * called `preventDefault()`, which the cancelable create-events use to decide
 * whether to fall back to a `router.push(...)`. Pass `{ cancelable: true }` for
 * those.
 */
export function dispatchFlamingo<K extends FlamingoEventName>(
  type: K,
  ...args: FlamingoDetail<K> extends undefined
    ? [init?: { cancelable?: boolean }]
    : [init: { detail: FlamingoDetail<K>; cancelable?: boolean }]
): boolean {
  return window.dispatchEvent(new CustomEvent(type, args[0]));
}
