/**
 * Decodes a single percent-encoded URL path segment captured from `usePathname()`.
 *
 * Returns the decoded string on success, or `null` when the input contains a
 * malformed percent-escape sequence (i.e. `decodeURIComponent` throws `URIError`).
 * Callers in layout-level components MUST handle the `null` return — swallowing
 * a `URIError` here prevents a crash that would escape every route segment's
 * `error.tsx` (see `.claude/rules/frontend-rsc-error-handling.md`).
 *
 * Safe to import from both Server Components and Client Components — this module
 * has no dependency on next/headers or any server-only API.
 */
export function safeDecodePathSegment(segment: string): string | null {
  try {
    return decodeURIComponent(segment);
  } catch {
    return null;
  }
}
