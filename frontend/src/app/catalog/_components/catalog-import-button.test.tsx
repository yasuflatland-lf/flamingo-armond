// @vitest-environment happy-dom
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { renderWithIntl } from "@/test/render-with-intl";
import { CatalogImportButton } from "./catalog-import-button";

const CARD = { id: "m-1" };

describe("<CatalogImportButton>", () => {
  it("renders the idle action label and stays enabled", () => {
    renderWithIntl(
      <CatalogImportButton card={CARD} importing={false} imported={false} onImport={vi.fn()} />,
    );

    const btn = screen.getByTestId("catalog-import-m-1");
    expect(btn).toBeEnabled();
    expect(btn).toHaveTextContent("Import");
  });

  it("shows the in-flight label and disables the button while importing", () => {
    renderWithIntl(
      <CatalogImportButton card={CARD} importing={true} imported={false} onImport={vi.fn()} />,
    );

    const btn = screen.getByTestId("catalog-import-m-1");
    expect(btn).toBeDisabled();
    expect(btn).toHaveTextContent("Importing...");
  });

  it("shows the imported label and disables the button once imported", () => {
    renderWithIntl(
      <CatalogImportButton card={CARD} importing={false} imported={true} onImport={vi.fn()} />,
    );

    const btn = screen.getByTestId("catalog-import-m-1");
    expect(btn).toBeDisabled();
    expect(btn).toHaveTextContent("Imported");
  });

  it("calls onImport with the deck id when the idle button is clicked", async () => {
    const user = userEvent.setup();
    const onImport = vi.fn();
    renderWithIntl(
      <CatalogImportButton card={CARD} importing={false} imported={false} onImport={onImport} />,
    );

    await user.click(screen.getByTestId("catalog-import-m-1"));

    expect(onImport).toHaveBeenCalledWith("m-1");
  });

  it("uses custom labels and a custom testId prefix when provided", () => {
    renderWithIntl(
      <CatalogImportButton
        card={CARD}
        importing={false}
        imported={false}
        onImport={vi.fn()}
        labels={{ action: "Start with this deck", inProgress: "Starting...", done: "Added" }}
        testIdPrefix="onboarding-deck"
      />,
    );

    const btn = screen.getByTestId("onboarding-deck-m-1");
    expect(btn).toHaveTextContent("Start with this deck");
    // The default catalog testId must NOT be present when a prefix override is given.
    expect(screen.queryByTestId("catalog-import-m-1")).toBeNull();
  });

  it("merges the passthrough className onto the rendered button", () => {
    renderWithIntl(
      <CatalogImportButton
        card={CARD}
        importing={false}
        imported={false}
        onImport={vi.fn()}
        className="mt-auto w-full"
      />,
    );

    const btn = screen.getByTestId("catalog-import-m-1");
    expect(btn).toHaveClass("mt-auto", "w-full");
  });
});
