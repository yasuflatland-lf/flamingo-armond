/**
 * Open-redirect guard: allow only internal paths (must start with "/" but not
 * "//"). Rejects missing values, protocol-relative URLs ("//evil.com"), and
 * any scheme-bearing URLs ("https://...").
 *
 * Safe to import from both Server Components and Client Components — this
 * module has no dependency on next/headers or any server-only API.
 */
export function sanitizeReturnTo(value: string | undefined): string | null {
  if (!value) return null;
  // reject "//" and "/\" -- both normalise to protocol-relative in browsers
  if (!value.startsWith("/") || value[1] === "/" || value[1] === "\\") return null;
  return value;
}
