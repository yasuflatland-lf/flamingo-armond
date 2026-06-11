import { cookies, headers } from "next/headers";
import { getRequestConfig } from "next-intl/server";
import { LOCALE_COOKIE } from "./config";
import { loadMessages } from "./load-messages";
import { resolveLocale } from "./negotiate";

/**
 * Resolves the active locale for every server render. Priority:
 *   1. NEXT_LOCALE cookie (the user's explicit language choice, when present)
 *   2. Accept-Language negotiation (first-visit auto-detect)
 *   3. defaultLocale ("en")
 * No middleware is involved; this runs in request scope and reads headers/cookies
 * directly. Returning `locale` here is what makes getLocale()/useLocale() work.
 */
export default getRequestConfig(async () => {
  const [cookieStore, headerStore] = await Promise.all([cookies(), headers()]);
  const locale = resolveLocale(
    cookieStore.get(LOCALE_COOKIE)?.value,
    headerStore.get("accept-language"),
  );

  return {
    locale,
    messages: await loadMessages(locale),
  };
});
