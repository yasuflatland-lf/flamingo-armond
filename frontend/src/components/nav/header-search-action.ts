// Exact-match routes that show the mobile header search trigger.
const SEARCH_EXACT = new Set<string>(["/cardgroups"]);
// Pattern routes (e.g. parameterized segments) that show the trigger.
// Mirrors the `[id]/edit` regexes in `header-create-action.ts` so the magnifier
// and the "+" appear together on the nested card-list screens.
const SEARCH_PATTERNS: RegExp[] = [
  /^\/cardgroups\/[^/]+\/edit(\/|$)/,
  /^\/admin\/masters\/[^/]+\/edit(\/|$)/,
];

/**
 * Maps the current pathname to whether the mobile header should show the
 * search (filter) trigger. Pure function — no React/Next imports, no side
 * effects. Mirrors `resolveHeaderCreateAction`'s exact-set + regex-array shape.
 *
 * Routing rules:
 * - `/cardgroups` -> true (the filter lives here)
 * - `/cardgroups/:id/edit` -> true (cardgroup cards search)
 * - `/admin/masters/:id/edit` -> true (master cards search)
 * - anything else -> false
 *
 * Built to extend: add routes to `SEARCH_EXACT` / `SEARCH_PATTERNS` (and wire
 * the page) when other list screens adopt the header-takeover filter.
 */
export function resolveHeaderSearchAction(pathname: string): boolean {
  if (SEARCH_EXACT.has(pathname)) return true;
  return SEARCH_PATTERNS.some((re) => re.test(pathname));
}
