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
  it("renders name and the grouped card-count stat with its unit", () => {
    renderItem(FULL_NODE);
    expect(screen.getByText("Business English")).toBeInTheDocument();
    expect(screen.getByText("1,245")).toBeInTheDocument();
    expect(screen.getByText("cards")).toBeInTheDocument();
  });

  it("renders the name and stat when the deck has no description", () => {
    renderItem(BARE_NODE);
    expect(screen.getByText("JLPT N3 Kanji")).toBeInTheDocument();
    expect(screen.getByText("100")).toBeInTheDocument();
  });

  it("renders the singular card unit when the deck has exactly one card", () => {
    const oneCardNode = makeFragmentData(
      {
        __typename: "MasterCardgroup" as const,
        id: "m-3",
        name: "Single Card Deck",
        description: null,
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
    expect(screen.getByText("›")).toHaveAttribute("aria-hidden", "true");
  });

  it("keeps the description in the DOM (revealed on hover/focus via CSS)", () => {
    renderItem(FULL_NODE);
    const desc = screen.getByTestId("catalog-row-desc-m-1");
    expect(desc).toHaveTextContent("Professional vocabulary");
    const revealWrapper = desc.closest("div.grid");
    expect(revealWrapper?.className).toContain("grid-rows-[0fr]");
    expect(revealWrapper?.className).toContain("opacity-0");
    expect(revealWrapper?.className).toContain("group-hover:grid-rows-[1fr]");
    expect(revealWrapper?.className).toContain("group-focus-within:grid-rows-[1fr]");
    expect(revealWrapper?.className).toContain("motion-reduce:transition-none");
  });

  it("renders no description node when the deck has none", () => {
    renderItem(BARE_NODE);
    expect(screen.queryByTestId("catalog-row-desc-m-2")).toBeNull();
  });
});
