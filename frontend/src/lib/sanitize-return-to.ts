/**
 * Open-redirect guard: returns a same-origin path (pathname + search + hash)
 * or null. Resolves the value with the WHATWG URL parser against a sentinel
 * origin, so parser-level normalisations (tab/LF/CR stripping, `\` → `/`,
 * dot-segment removal) are applied before the origin check.
 *
 * Safe to import from both Server Components and Client Components — this
 * module has no dependency on next/headers or any server-only API.
 */
const SENTINEL_ORIGIN = "http://internal.invalid";

export function sanitizeReturnTo(value: string | undefined): string | null {
  // Not a positional "//" check: "evil.com" / "?x" / "#h" would otherwise
  // resolve to "/evil.com" etc. and be accepted as relative references.
  if (!value?.startsWith("/")) return null;
  let url: URL;
  try {
    url = new URL(value, SENTINEL_ORIGIN);
  } catch {
    return null;
  }
  if (url.origin !== SENTINEL_ORIGIN) return null;
  const path = url.pathname + url.search + url.hash;
  // "/..//evil.com" resolves to pathname "//evil.com" on the sentinel origin;
  // returning it verbatim would hand the caller a protocol-relative URL.
  if (path.startsWith("//") || path.startsWith("/\\")) return null;
  return path;
}
