// @vitest-environment jsdom
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { renderWithIntl } from "@/test/render-with-intl";
import { type CatalogDeck, CatalogDeckHeader } from "./catalog-deck-header";

const FULL_DECK: CatalogDeck = {
  __typename: "MasterCardgroup",
  id: "deck-1",
  name: "Business English",
  description: "Professional vocabulary",
};

const BARE_DECK: CatalogDeck = {
  __typename: "MasterCardgroup",
  id: "deck-2",
  name: "JLPT N3 Kanji",
  description: null,
};

function renderHeader(
  deck: CatalogDeck,
  props: Partial<{
    cardCount: number;
    importing: boolean;
    imported: boolean;
    onImport: (id: string) => void;
  }> = {},
) {
  const onImport = props.onImport ?? vi.fn();
  renderWithIntl(
    <CatalogDeckHeader
      deck={deck}
      cardCount={props.cardCount ?? 42}
      importing={props.importing ?? false}
      imported={props.imported ?? false}
      onImport={onImport}
    />,
  );
  return { onImport };
}

describe("<CatalogDeckHeader>", () => {
  it("renders the deck name as the h1 title and the card count", () => {
    renderHeader(FULL_DECK);
    expect(screen.getByRole("heading", { level: 1, name: "Business English" })).toBeInTheDocument();
    expect(screen.getByText("42 cards")).toBeInTheDocument();
  });

  it("renders the description", () => {
    renderHeader(FULL_DECK);
    expect(screen.getByTestId("catalog-deck-description")).toHaveTextContent(
      "Professional vocabulary",
    );
  });

  it("omits the description when it is null", () => {
    renderHeader(BARE_DECK);
    expect(screen.queryByTestId("catalog-deck-description")).toBeNull();
  });

  it("renders a back link to /catalog", () => {
    renderHeader(FULL_DECK);
    expect(screen.getByRole("link", { name: "Back to catalog" })).toHaveAttribute(
      "href",
      "/catalog",
    );
  });

  it("renders the import CTA with a locale-independent testId and calls onImport on click", async () => {
    const user = userEvent.setup();
    const { onImport } = renderHeader(FULL_DECK);
    const btn = screen.getByTestId("catalog-deck-import-deck-1");
    expect(btn).toBeInTheDocument();
    await user.click(btn);
    expect(onImport).toHaveBeenCalledWith("deck-1");
  });

  it("disables the import CTA and shows the in-flight label while importing", () => {
    renderHeader(FULL_DECK, { importing: true });
    const btn = screen.getByTestId("catalog-deck-import-deck-1");
    expect(btn).toBeDisabled();
    expect(btn).toHaveTextContent("Importing...");
  });

  it("disables the import CTA and shows the imported label once imported", () => {
    renderHeader(FULL_DECK, { imported: true });
    const btn = screen.getByTestId("catalog-deck-import-deck-1");
    expect(btn).toBeDisabled();
    expect(btn).toHaveTextContent("Imported");
  });

  it("renders the import CTA full-width below the deck description, not in the app-bar", () => {
    renderHeader(FULL_DECK);
    const btn = screen.getByTestId("catalog-deck-import-deck-1");
    // The CTA spans the content width rather than sitting in the right-aligned
    // app-bar action cluster.
    expect(btn).toHaveClass("w-full");
    // It follows the description in document order (below the header content).
    const description = screen.getByTestId("catalog-deck-description");
    expect(
      description.compareDocumentPosition(btn) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });

  it("keeps the import CTA full-width on a bare deck with no description", () => {
    renderHeader(BARE_DECK);
    expect(screen.getByTestId("catalog-deck-import-deck-2")).toHaveClass("w-full");
  });
});
