"use server";

import { cookies } from "next/headers";
import { LOCALE_COOKIE, type Locale } from "./config";

/**
 * Persists the user's explicit language choice in the `NEXT_LOCALE` cookie. The
 * cookie is read by `next-intl`'s request config on the next render to resolve
 * the active locale. A one-year `maxAge` keeps the choice across sessions; the
 * `lax` SameSite policy keeps the cookie scoped to first-party navigations.
 */
export async function setUserLocale(locale: Locale): Promise<void> {
  (await cookies()).set(LOCALE_COOKIE, locale, {
    path: "/",
    maxAge: 60 * 60 * 24 * 365,
    sameSite: "lax",
  });
}
