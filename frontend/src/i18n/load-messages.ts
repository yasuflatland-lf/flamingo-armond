import { defaultLocale, type Locale } from "./config";

/**
 * Load the message catalog for a locale, falling back to the default-locale
 * catalog when the requested one is missing. A locale added to `locales` without
 * its `messages/<locale>.json` would otherwise reject inside `getRequestConfig`
 * and 500 every route; this degrades to the default catalog (always present) and
 * logs, so the failure is recoverable and diagnosable instead of a site outage.
 */
export async function loadMessages(locale: Locale) {
  try {
    return (await import(`../../messages/${locale}.json`)).default;
  } catch (error) {
    console.error(
      "[i18n] missing message catalog for locale, falling back to default:",
      locale,
      error,
    );
    return (await import(`../../messages/${defaultLocale}.json`)).default;
  }
}
