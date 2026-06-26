// @vitest-environment happy-dom

import type { MockedResponse } from "@apollo/client/testing";
import { MockedProvider } from "@apollo/client/testing/react";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { MasterCatalogDocument } from "@/generated/graphql";
import { renderWithIntl } from "@/test/render-with-intl";
import { CardgroupCardsSection } from "./cardgroup-cards-section";

const onAddCard = vi.fn();
const onBatchImport = vi.fn();

// Variables mirror CATALOG_DEFAULT_VARS ({ first: 20, search: null }) from
// merge-from-catalog-sheet.tsx. The sheet passes `skip: !open`, so this mock
// is only consumed when the merge item is clicked and the sheet opens.
const BASE_CATALOG: MockedResponse = {
  request: {
    query: MasterCatalogDocument,
    variables: { first: 20, search: null },
  },
  maxUsageCount: Number.POSITIVE_INFINITY,
  result: {
    data: {
      masterCatalog: {
        __typename: "MasterCatalogConnection",
        totalCount: 1,
        edges: [
          {
            __typename: "MasterCatalogEdge",
            cursor: "cursor-master-1",
            node: {
              __typename: "MasterCardgroup",
              id: "master-1",
              name: "Business English",
              description: "Professional vocabulary",
              cardCount: 42,
            },
          },
        ],
        pageInfo: {
          __typename: "PageInfo",
          hasNextPage: false,
          hasPreviousPage: false,
          startCursor: "cursor-master-1",
          endCursor: "cursor-master-1",
        },
      },
    },
  },
};

// Stub CardsClient so this test focuses on the section header (the only piece
// CardgroupCardsSection actually owns) and how the render-prop is invoked.
vi.mock("@/app/cardgroups/[id]/cards/cards-client", () => ({
  CardsClient: ({
    sectionHeader,
  }: {
    sectionHeader?:
      | ReactNode
      | ((args: {
          totalCount: number;
          onAddCard: () => void;
          onBatchImport: () => void;
        }) => ReactNode);
  }) => (
    <div data-testid="cards-client-stub">
      {typeof sectionHeader === "function"
        ? sectionHeader({ totalCount: 12, onAddCard, onBatchImport })
        : sectionHeader}
    </div>
  ),
}));

const PAGE_INFO = {
  __typename: "PageInfo" as const,
  hasNextPage: false,
  hasPreviousPage: false,
  startCursor: null,
  endCursor: null,
};

function renderSection(
  cardgroupId = "cg-1",
  initialTotalCount = 7,
  renderPageHeader?: (args: {
    totalCount: number;
    onBatchImport: () => void;
    onMerge: () => void;
  }) => ReactNode,
  extraMocks: MockedResponse[] = [],
) {
  renderWithIntl(
    <MockedProvider mocks={[BASE_CATALOG, ...extraMocks]}>
      <CardgroupCardsSection
        cardgroupId={cardgroupId}
        cardgroupName="Test Cardgroup"
        initialEdges={[]}
        initialPageInfo={PAGE_INFO}
        initialTotalCount={initialTotalCount}
        renderPageHeader={renderPageHeader}
      />
    </MockedProvider>,
  );
}

beforeEach(() => {
  onAddCard.mockClear();
  onBatchImport.mockClear();
});

describe("<CardgroupCardsSection>", () => {
  it("renders the toolbar buttons without a count chip", () => {
    // The count chip was removed — count is now shown in the page-level Badge
    // via the renderPageHeader render prop. The toolbar contains only Start
    // learning and Add card split button.
    renderSection("cg-1", 7);
    expect(screen.getByRole("link", { name: /start learning/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /add card/i })).toBeInTheDocument();
    // No h2 "Cards (N)" heading in the toolbar row.
    expect(screen.queryByRole("heading", { name: /cards \(/i })).not.toBeInTheDocument();
  });

  it("invokes renderPageHeader with the live totalCount from CardsClient", () => {
    // The stub passes totalCount: 12 — covers the live cache value.
    renderSection("cg-1", 7, ({ totalCount }) => (
      <div data-testid="page-header-slot">{totalCount} cards</div>
    ));
    // The page-level slot receives the live count, not the SSR seed (7).
    expect(screen.getByTestId("page-header-slot")).toHaveTextContent("12 cards");
  });

  it("does not render the page header slot when renderPageHeader is omitted", () => {
    renderSection("cg-1", 7);
    expect(screen.queryByTestId("page-header-slot")).not.toBeInTheDocument();
  });

  it("renders a Start learning link to /learn/:id", () => {
    renderSection("cg-1");
    const link = screen.getByRole("link", { name: /start learning/i });
    expect(link).toHaveAttribute("href", "/learn/cg-1");
  });

  it("primary Add card button calls onAddCard", async () => {
    const user = userEvent.setup();
    renderSection("cg-1");
    await user.click(screen.getByRole("button", { name: /add card \+/i }));
    expect(onAddCard).toHaveBeenCalledTimes(1);
  });

  it("dropdown 'Add a card' item calls onAddCard", async () => {
    const user = userEvent.setup();
    renderSection("cg-1");
    await user.click(screen.getByRole("button", { name: /more add options/i }));
    const addItem = await screen.findByRole("menuitem", { name: /add a card/i });
    await user.click(addItem);
    expect(onAddCard).toHaveBeenCalledTimes(1);
  });

  it("dropdown 'Batch import' item calls onBatchImport", async () => {
    const user = userEvent.setup();
    renderSection("cg-1");
    await user.click(screen.getByRole("button", { name: /more add options/i }));
    const batchItem = await screen.findByRole("menuitem", { name: /batch import/i });
    await user.click(batchItem);
    expect(onBatchImport).toHaveBeenCalledTimes(1);
  });

  it("URL-encodes ampersand characters in the cardgroup id for the Start learning link", () => {
    renderSection("cg&evil");
    expect(screen.getByRole("link", { name: /start learning/i })).toHaveAttribute(
      "href",
      "/learn/cg%26evil",
    );
  });

  it("forwards the section header into the CardsClient sectionHeader slot", () => {
    renderSection();
    const stub = screen.getByTestId("cards-client-stub");
    expect(stub).toContainElement(screen.getByRole("link", { name: /start learning/i }));
    expect(stub).toContainElement(screen.getByRole("button", { name: /add card \+/i }));
  });

  it("forwards onBatchImport into the renderPageHeader slot", async () => {
    // The mobile batch-import control now lives in the page header's overflow
    // menu, so the section must thread CardsClient's onBatchImport up to the
    // renderPageHeader render prop, not just into its own toolbar.
    const user = userEvent.setup();
    renderSection("cg-1", 7, ({ onBatchImport: headerImport }) => (
      <button type="button" data-testid="page-header-import" onClick={headerImport}>
        header import
      </button>
    ));
    await user.click(screen.getByTestId("page-header-import"));
    expect(onBatchImport).toHaveBeenCalledTimes(1);
  });

  it("forwards onMerge into the renderPageHeader slot and it opens the merge sheet", async () => {
    // The mobile merge control lives in the page header's overflow menu, so the
    // section must thread its onMerge callback up to the renderPageHeader prop.
    const user = userEvent.setup();
    renderSection("cg-1", 7, ({ onMerge: headerMerge }) => (
      <button type="button" data-testid="page-header-merge" onClick={headerMerge}>
        header merge
      </button>
    ));
    await user.click(screen.getByTestId("page-header-merge"));
    // onMerge is () => setMergeOpen(true), owned by the section, so clicking
    // the slot's button opens the merge sheet.
    await waitFor(() => {
      expect(screen.getByTestId("merge-from-catalog-search")).toBeInTheDocument();
    });
  });

  it("desktop split-button 'Merge from catalog' item opens the merge sheet", async () => {
    const user = userEvent.setup();
    renderSection("cg-1");
    // Open the split-button dropdown
    await user.click(screen.getByRole("button", { name: /more add options/i }));
    const mergeItem = await screen.findByTestId("cardgroup-merge-menuitem");
    expect(mergeItem).toBeInTheDocument();
    await user.click(mergeItem);
    // The merge sheet is now owned by CardgroupCardsSection; clicking the menu
    // item sets mergeOpen=true which renders the sheet with the catalog search.
    await waitFor(() => {
      expect(screen.getByTestId("merge-from-catalog-search")).toBeInTheDocument();
    });
  });
});
