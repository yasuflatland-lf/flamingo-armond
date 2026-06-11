import { defaultLocale, type Locale, toLocale } from "./config";

type Ranked = { tag: string; q: number };

/**
 * Pick the best supported Locale from an Accept-Language header value.
 * Parses `tag;q=` weights, drops tags explicitly rejected with `q=0` (RFC 7231),
 * sorts by q desc (stable for ties), and returns the first tag whose primary
 * subtag matches a supported locale. Falls back to the default when nothing matches.
 */
export function negotiateLocale(acceptLanguage: string | null | undefined): Locale {
  if (!acceptLanguage) return defaultLocale;

  const ranked: Ranked[] = acceptLanguage
    .split(",")
    .map((part) => {
      const segments = part.trim().split(";");
      const tag = (segments[0] ?? "").toLowerCase();
      const qParam = segments.slice(1).find((p) => p.trim().startsWith("q="));
      const q = qParam ? Number.parseFloat(qParam.trim().slice(2)) : 1;
      return { tag, q: Number.isFinite(q) ? q : 0 };
    })
    .filter((r) => r.tag.length > 0 && r.q > 0)
    .sort((a, b) => b.q - a.q);

  for (const { tag } of ranked) {
    const primary = toLocale(tag.split("-")[0]);
    if (primary) return primary;
  }
  return defaultLocale;
}

/**
 * Resolve the active locale from the persisted cookie value and the request's
 * Accept-Language header. A valid cookie wins; an absent or unsupported cookie
 * value falls through to Accept-Language negotiation (and ultimately the default).
 * Pure function — no next/headers access — so the precedence is unit-testable
 * without a request context.
 */
export function resolveLocale(
  cookieValue: string | null | undefined,
  acceptLanguage: string | null | undefined,
): Locale {
  return toLocale(cookieValue) ?? negotiateLocale(acceptLanguage);
}
