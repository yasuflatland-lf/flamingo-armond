// @vitest-environment happy-dom

import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeAll, describe, expect, it, vi } from "vitest";
import { renderWithIntl } from "@/test/render-with-intl";
import enMessages from "../../../messages/en.json";
import { LanguageSwitcher } from "./language-switcher";

// Read the Japanese option label from the catalog rather than inlining a CJK
// literal — the language policy forbids non-English string literals in source.
const japaneseLabel = enMessages.Language.ja;

const setUserLocale = vi.hoisted(() => vi.fn());

vi.mock("@/i18n/locale-actions", () => ({
  setUserLocale,
}));

// Radix Select drives its popup through pointer-capture and scroll APIs that
// jsdom does not implement. Polyfill the no-ops so the listbox opens under a
// real `userEvent.click` instead of forcing the test to bypass the component.
beforeAll(() => {
  if (!Element.prototype.hasPointerCapture) {
    Element.prototype.hasPointerCapture = () => false;
  }
  if (!Element.prototype.setPointerCapture) {
    Element.prototype.setPointerCapture = () => {};
  }
  if (!Element.prototype.releasePointerCapture) {
    Element.prototype.releasePointerCapture = () => {};
  }
  if (!Element.prototype.scrollIntoView) {
    Element.prototype.scrollIntoView = () => {};
  }
});

describe("LanguageSwitcher", () => {
  it("renders the Language label and the current locale value", () => {
    renderWithIntl(<LanguageSwitcher />);

    // The accessible name of the combobox comes from the associated label text.
    expect(screen.getByText("Language")).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: /language/i })).toBeInTheDocument();
    // Default locale in renderWithIntl is `en`, so the trigger shows "English".
    expect(screen.getByText("English")).toBeInTheDocument();
  });

  it("calls setUserLocale('ja') when the Japanese option is selected", async () => {
    const user = userEvent.setup();
    renderWithIntl(<LanguageSwitcher />);

    await user.click(screen.getByRole("combobox", { name: /language/i }));

    const japaneseOption = await screen.findByRole("option", { name: japaneseLabel });
    await user.click(japaneseOption);

    await waitFor(() => {
      expect(setUserLocale).toHaveBeenCalledWith("ja");
    });
  });
});
