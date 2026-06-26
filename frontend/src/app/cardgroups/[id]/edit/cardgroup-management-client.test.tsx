// @vitest-environment happy-dom
import { screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { renderWithIntl } from "@/test/render-with-intl";
import { CardgroupManagementClient } from "./cardgroup-management-client";

// Stub the children so this test focuses on layout chrome.
// CardgroupHeader mutation behaviour is covered by cardgroup-header.test.tsx.
// CardgroupCardsSection behaviour is covered by cardgroup-cards-section.test.tsx.
vi.mock("@/components/cardgroups/cardgroup-header", () => ({
  CardgroupHeader: ({
    cardgroup,
    totalCount,
  }: {
    cardgroup: { id: string; name: string };
    totalCount: number;
  }) => (
    <div data-testid="header-stub">
      <h1>{cardgroup.name}</h1>
      <span data-testid="header-total">{totalCount}</span>
    </div>
  ),
}));
vi.mock("@/components/cardgroups/cardgroup-cards-section", () => ({
  CardgroupCardsSection: ({
    cardgroupId,
    initialTotalCount,
    renderPageHeader,
  }: {
    cardgroupId: string;
    initialTotalCount: number;
    renderPageHeader?: (args: { totalCount: number }) => React.ReactNode;
  }) => (
    <div data-testid="cards-section-stub">
      {renderPageHeader ? renderPageHeader({ totalCount: initialTotalCount }) : null}
      <span data-testid="cards-section-id">{cardgroupId}</span>
      <span data-testid="cards-section-total">{initialTotalCount}</span>
    </div>
  ),
}));

const CARDGROUP = { id: "cg-1", name: "Spanish Vocab" };
const PAGE_INFO = {
  __typename: "PageInfo" as const,
  hasNextPage: false,
  hasPreviousPage: false,
  startCursor: null,
  endCursor: null,
};

describe("<CardgroupManagementClient>", () => {
  it("renders the cardgroup name as the h1 page title", () => {
    renderWithIntl(
      <CardgroupManagementClient
        cardgroup={CARDGROUP}
        initialEdges={[]}
        initialPageInfo={PAGE_INFO}
        initialTotalCount={0}
      />,
    );
    expect(screen.getByRole("heading", { level: 1, name: /spanish vocab/i })).toBeInTheDocument();
  });

  it("passes totalCount from renderPageHeader to CardgroupHeader via the render prop", () => {
    renderWithIntl(
      <CardgroupManagementClient
        cardgroup={CARDGROUP}
        initialEdges={[]}
        initialPageInfo={PAGE_INFO}
        initialTotalCount={5}
      />,
    );
    // The stub invokes renderPageHeader with the initialTotalCount value,
    // and the CardgroupHeader stub renders it as data-testid="header-total".
    expect(screen.getByTestId("header-total")).toHaveTextContent("5");
    expect(screen.getByTestId("cards-section-id")).toHaveTextContent("cg-1");
  });

  it("uses a single <main> landmark", () => {
    renderWithIntl(
      <CardgroupManagementClient
        cardgroup={CARDGROUP}
        initialEdges={[]}
        initialPageInfo={PAGE_INFO}
        initialTotalCount={0}
      />,
    );
    expect(screen.getAllByRole("main")).toHaveLength(1);
  });
});
