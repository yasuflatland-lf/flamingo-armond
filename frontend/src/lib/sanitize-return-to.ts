const SENTINEL_ORIGIN = "http://internal.invalid";

/**
 * Open-redirect guard: returns a same-origin path (pathname + search + hash)
 * or null. Resolves the value with the WHATWG URL parser against a sentinel
 * origin, so parser-level normalisations (tab/LF/CR stripping, `\` → `/`,
 * dot-segment removal) are applied before the origin check.
 *
 * Safe to import from both Server Components and Client Components — this
 * module has no dependency on next/headers or any server-only API.
 */
export function sanitizeReturnTo(value: string | undefined): string | null {
  // Keep the leading-"/" precheck: without it "evil.com" / "?x" / "#h" resolve
  // to "/evil.com" etc. and are accepted as relative references. Not
  // `value?.startsWith`: it throws on the string[] a page receives for a
  // repeated query key.
  if (typeof value !== "string" || !value.startsWith("/")) return null;
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
