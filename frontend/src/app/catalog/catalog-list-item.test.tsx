// @vitest-environment jsdom
import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { CatalogCardFieldsFragment } from "@/app/catalog/queries";
import { makeFragmentData } from "@/generated/fragment-masking";
import { renderWithIntl } from "@/test/render-with-intl";
import { CatalogListItem } from "./catalog-list-item";

// `makeFragmentData` is identity at runtime, so the wrapped object still carries
// every field the component reads via `useFragment`; the wrap only supplies the
// masked `FragmentType` the `node` prop expects.
const FULL_NODE = makeFragmentData(
  {
    __typename: "MasterCardgroup" as const,
    id: "m-1",
    name: "Business English",
    description: "Professional vocabulary",
    language: "en",
    level: "B2",
    category: "Business",
    cardCount: 1245,
  },
  CatalogCardFieldsFragment,
);

const BARE_NODE = makeFragmentData(
  {
    __typename: "MasterCardgroup" as const,
    id: "m-2",
    name: "JLPT N3 Kanji",
    description: null,
    language: null,
    level: null,
    category: null,
    cardCount: 100,
  },
  CatalogCardFieldsFragment,
);

function renderItem(node: typeof FULL_NODE) {
  renderWithIntl(
    <ul>
      <CatalogListItem node={node} />
    </ul>,
  );
}

describe("<CatalogListItem>", () => {
  it("renders name, the grouped card-count stat with its unit, and badges", () => {
    renderItem(FULL_NODE);
    expect(screen.getByText("Business English")).toBeInTheDocument();
    expect(screen.getByText("1,245")).toBeInTheDocument();
    expect(screen.getByText("cards")).toBeInTheDocument();
    expect(screen.getByText("en")).toBeInTheDocument();
    expect(screen.getByText("Level B2")).toBeInTheDocument();
    expect(screen.getByText("Business")).toBeInTheDocument();
  });

  it("renders the name and stat but omits badges when those fields are null", () => {
    renderItem(BARE_NODE);
    expect(screen.getByText("JLPT N3 Kanji")).toBeInTheDocument();
    expect(screen.getByText("100")).toBeInTheDocument();
    expect(screen.queryByText(/^Level /)).not.toBeInTheDocument();
    expect(screen.queryByText("Business")).not.toBeInTheDocument();
  });

  it("renders the singular card unit when the deck has exactly one card", () => {
    const oneCardNode = makeFragmentData(
      {
        __typename: "MasterCardgroup" as const,
        id: "m-3",
        name: "Single Card Deck",
        description: null,
        language: null,
        level: null,
        category: null,
        cardCount: 1,
      },
      CatalogCardFieldsFragment,
    );
    renderWithIntl(
      <ul>
        <CatalogListItem node={oneCardNode} />
      </ul>,
    );
    expect(screen.getByText("1")).toBeInTheDocument();
    expect(screen.getByText("card")).toBeInTheDocument();
  });

  it("renders the whole row as a single link to the deck-detail page", () => {
    renderItem(FULL_NODE);
    const row = screen.getByTestId("catalog-row-m-1");
    expect(row).toHaveAttribute("href", "/catalog/m-1");
    expect(row).toHaveAttribute("aria-label", "View Business English");
  });

  it("renders no Import or View button (actions moved to the detail page)", () => {
    renderItem(FULL_NODE);
    expect(screen.queryByRole("button")).toBeNull();
    expect(screen.queryByTestId("catalog-import-m-1")).toBeNull();
    expect(screen.queryByTestId("catalog-view-m-1")).toBeNull();
  });

  it("keeps the description in the DOM (revealed on hover/focus via CSS)", () => {
    renderItem(FULL_NODE);
    expect(screen.getByText("Professional vocabulary")).toBeInTheDocument();
  });

  it("renders no description node when the deck has none", () => {
    renderItem(BARE_NODE);
    expect(screen.queryByTestId("catalog-row-desc-m-2")).toBeNull();
  });
});
