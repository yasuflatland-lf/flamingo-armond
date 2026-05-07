// @vitest-environment jsdom

import { render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import { CardgroupCardsSection } from "./cardgroup-cards-section";

// Stub CardsClient so this test focuses on the section header (the only piece
// CardgroupCardsSection actually owns) and how the render-prop is invoked.
vi.mock("@/app/cardgroups/[id]/cards/cards-client", () => ({
  CardsClient: ({
    sectionHeader,
  }: {
    sectionHeader?: ReactNode | ((args: { totalCount: number }) => ReactNode);
  }) => (
    <div data-testid="cards-client-stub">
      {typeof sectionHeader === "function" ? sectionHeader({ totalCount: 12 }) : sectionHeader}
    </div>
  ),
}));

const PAGE_INFO = {
  hasNextPage: false,
  hasPreviousPage: false,
  startCursor: null,
  endCursor: null,
};

function renderSection(
  cardgroupId = "cg-1",
  initialTotalCount = 7,
  renderPageHeader?: (args: { totalCount: number }) => ReactNode,
) {
  render(
    <CardgroupCardsSection
      cardgroupId={cardgroupId}
      initialEdges={[]}
      initialPageInfo={PAGE_INFO}
      initialTotalCount={initialTotalCount}
      renderPageHeader={renderPageHeader}
    />,
  );
}

describe("<CardgroupCardsSection>", () => {
  it("renders the toolbar buttons without a count chip", () => {
    // The count chip was removed — count is now shown in the page-level Badge
    // via the renderPageHeader render prop. The toolbar contains only Start
    // learning and Add card.
    renderSection("cg-1", 7);
    expect(screen.getByRole("link", { name: /start learning/i })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /add card/i })).toBeInTheDocument();
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

  it("renders an Add card link with cardgroup and return params", () => {
    renderSection("cg-1");
    const link = screen.getByRole("link", { name: /add card/i });
    expect(link).toHaveAttribute("href", "/cards/new?cardgroup=cg-1&return=/cardgroups/cg-1/edit");
  });

  it("URL-encodes ampersand characters in the cardgroup id for both links", () => {
    renderSection("cg&evil");
    expect(screen.getByRole("link", { name: /start learning/i })).toHaveAttribute(
      "href",
      "/learn/cg%26evil",
    );
    expect(screen.getByRole("link", { name: /add card/i })).toHaveAttribute(
      "href",
      "/cards/new?cardgroup=cg%26evil&return=/cardgroups/cg%26evil/edit",
    );
  });

  it("forwards the section header into the CardsClient sectionHeader slot", () => {
    renderSection();
    const stub = screen.getByTestId("cards-client-stub");
    expect(stub).toContainElement(screen.getByRole("link", { name: /start learning/i }));
    expect(stub).toContainElement(screen.getByRole("link", { name: /add card/i }));
  });
});
