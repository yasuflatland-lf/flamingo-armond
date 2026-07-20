// @vitest-environment happy-dom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import enMessages from "../../../../messages/en.json";
import jaMessages from "../../../../messages/ja.json";
import Loading from "./loading";

// The route-segment fallback is a server component, so the catalog is reached
// through next-intl/server rather than NextIntlClientProvider. The mock is
// locale-switchable so the heading can be asserted in both catalogs.
let activeMessages: typeof enMessages = enMessages;
vi.mock("next-intl/server", () => ({
  getTranslations: vi.fn(async (namespace: "Cards") => (key: string) => {
    const value = (activeMessages[namespace] as Record<string, string>)[key];
    return value ?? key;
  }),
}));

describe("/cards/new <Loading>", () => {
  it("renders the same localized heading key the resolved page renders", async () => {
    activeMessages = enMessages;
    render(await Loading());

    expect(
      screen.getByRole("heading", { name: enMessages.Cards.newCardTitle }),
    ).toBeInTheDocument();
  });

  it("renders the heading in the active locale so it does not swap on hydration", async () => {
    activeMessages = jaMessages;
    render(await Loading());

    expect(
      screen.getByRole("heading", { name: jaMessages.Cards.newCardTitle }),
    ).toBeInTheDocument();
  });
});
