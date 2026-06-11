"use server";

import { cookies } from "next/headers";
import { LOCALE_COOKIE, type Locale, toLocale } from "./config";

/**
 * Persists the user's explicit language choice in the `NEXT_LOCALE` cookie. The
 * cookie is read by `next-intl`'s request config on the next render to resolve
 * the active locale. A one-year `maxAge` keeps the choice across sessions; the
 * `lax` SameSite policy keeps the cookie scoped to first-party navigations.
 *
 * A `"use server"` action compiles to a network-reachable POST endpoint, so the
 * compile-time `Locale` parameter is not a runtime guarantee — the value is
 * re-narrowed via `toLocale` and an unsupported value is ignored rather than
 * written verbatim into the cookie.
 */
export async function setUserLocale(locale: Locale): Promise<void> {
  const safeLocale = toLocale(locale);
  if (safeLocale === null) {
    console.warn("[i18n] setUserLocale: ignoring unsupported locale value:", locale);
    return;
  }
  try {
    (await cookies()).set(LOCALE_COOKIE, safeLocale, {
      path: "/",
      maxAge: 60 * 60 * 24 * 365,
      sameSite: "lax",
      secure: process.env.NODE_ENV === "production",
    });
  } catch (error) {
    console.error("[i18n] setUserLocale: failed to write NEXT_LOCALE cookie:", error);
    throw error;
  }
}
