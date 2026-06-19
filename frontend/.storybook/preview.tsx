import type { Decorator, Preview } from "@storybook/nextjs-vite";
import { NextIntlClientProvider } from "next-intl";
import type { ReactElement } from "react";
import enMessages from "../messages/en.json";
import jaMessages from "../messages/ja.json";
import "../src/app/globals.css";

const messagesByLocale = { en: enMessages, ja: jaMessages } as const;
type StoryLocale = keyof typeof messagesByLocale;

const withIntl: Decorator = (Story, context): ReactElement => {
  const locale = (context.globals.locale as StoryLocale) ?? "en";
  return (
    <NextIntlClientProvider locale={locale} messages={messagesByLocale[locale]} timeZone="UTC">
      <Story />
    </NextIntlClientProvider>
  );
};

const preview: Preview = {
  decorators: [withIntl],
  globalTypes: {
    locale: {
      description: "UI locale",
      defaultValue: "en",
      toolbar: {
        icon: "globe",
        items: [
          { value: "en", title: "English" },
          { value: "ja", title: "Japanese" },
        ],
        dynamicTitle: true,
      },
    },
  },
  parameters: {
    // Phase 1: surface a11y violations in the UI panel without failing CI.
    a11y: { test: "todo" },
  },
};

export default preview;
