// Supported UI locales. `en` is the default; `ja` is the added locale. Exported
// so a consumer that enumerates locales (e.g. the language switcher) can map
// over the canonical list rather than re-declaring it.
export const locales = ["en", "ja"] as const;

export type Locale = (typeof locales)[number];

/** Fallback when no cookie is set and Accept-Language matches nothing. */
export const defaultLocale: Locale = "en";

/** Name of the cookie that persists the user's explicit language choice. */
export const LOCALE_COOKIE = "NEXT_LOCALE";

/**
 * Open Graph `og:locale` value per supported locale. Typed `Record<Locale, …>`
 * so adding a locale to `locales` forces an entry here (no silent en_US default).
 */
export const ogLocale: Record<Locale, string> = {
  en: "en_US",
  ja: "ja_JP",
};

/** Narrow an arbitrary string to a supported Locale, or null if unsupported. */
export function toLocale(value: string | undefined | null): Locale | null {
  return value != null && (locales as readonly string[]).includes(value) ? (value as Locale) : null;
}
