/**
 * Maps the current pathname to whether the mobile header should show the
 * search (filter) trigger. Pure function — no React/Next imports, no side
 * effects. Mirrors `resolveHeaderCreateAction`.
 *
 * Routing rules:
 * - `/cardgroups` -> true (the filter lives here)
 * - anything else -> false
 *
 * Built to extend: add routes here (and wire the page) when other list
 * screens adopt the header-takeover filter.
 */
export function resolveHeaderSearchAction(pathname: string): boolean {
  return pathname === "/cardgroups";
}
