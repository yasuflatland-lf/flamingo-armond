import { type RenderOptions, render } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import type { ReactElement, ReactNode } from "react";
import { defaultLocale, type Locale } from "@/i18n/config";
import enMessages from "../../messages/en.json";

type IntlMessages = typeof enMessages;

function Wrapper(
  { locale, messages }: { locale: Locale; messages: IntlMessages },
  { children }: { children: ReactNode },
) {
  return (
    <NextIntlClientProvider locale={locale} messages={messages} timeZone="UTC">
      {children}
    </NextIntlClientProvider>
  );
}

/**
 * Renders a component inside NextIntlClientProvider with the English catalog so
 * `t(...)` resolves to the original English copy. Existing string assertions
 * stay valid. Pass `locale="ja"` + `messages` to test Japanese rendering.
 */
export function renderWithIntl(
  ui: ReactElement,
  {
    locale = defaultLocale,
    messages = enMessages,
    ...options
  }: { locale?: Locale; messages?: IntlMessages } & Omit<RenderOptions, "wrapper"> = {},
) {
  return render(ui, { wrapper: Wrapper.bind(null, { locale, messages }), ...options });
}
