// Exact-match routes that show the mobile header search trigger.
const SEARCH_EXACT = new Set<string>(["/cardgroups", "/catalog", "/admin/masters", "/admin/users"]);
// Pattern routes (e.g. parameterized segments) that show the trigger.
// The `[id]/edit` regexes mirror `header-create-action.ts` so the magnifier and
// the "+" appear together on the nested card-list screens. The catalog
// deck-detail regex has NO `/edit` segment because the public catalog renders
// its card list directly at `/catalog/[id]` (a read-only screen with no separate
// edit route), so the detail route itself is the searchable card list.
const SEARCH_PATTERNS: RegExp[] = [
  /^\/cardgroups\/[^/]+\/edit(\/|$)/,
  /^\/admin\/masters\/[^/]+\/edit(\/|$)/,
  /^\/catalog\/[^/]+(\/|$)/,
];

/**
 * Maps the current pathname to whether the mobile header should show the
 * search (filter) trigger. Pure function — no React/Next imports, no side
 * effects. Mirrors `resolveHeaderCreateAction`'s exact-set + regex-array shape.
 *
 * Routing rules:
 * - `/cardgroups`, `/catalog`, `/admin/masters`, `/admin/users` -> true
 *   (each list screen wires the header-takeover filter)
 * - `/cardgroups/:id/edit`, `/admin/masters/:id/edit` -> true
 *   (the nested card-list screens)
 * - `/catalog/:id` -> true
 *   (the public catalog deck-detail screen renders its card list directly, with
 *   no nested /edit route, and wires the header-takeover filter)
 * - anything else -> false
 *
 * Built to extend: add routes to `SEARCH_EXACT` / `SEARCH_PATTERNS` (and wire
 * the page) when other list screens adopt the header-takeover filter.
 */
export function shouldShowHeaderSearch(pathname: string): boolean {
  if (SEARCH_EXACT.has(pathname)) return true;
  return SEARCH_PATTERNS.some((re) => re.test(pathname));
}
