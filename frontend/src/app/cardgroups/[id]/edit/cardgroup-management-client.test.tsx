// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { CardgroupManagementClient } from "./cardgroup-management-client";

// Stub the two children so this test focuses on layout and chrome
// (header, Back link, h1, slot composition). Mutation behaviour is
// covered by CardgroupSettingsCard's own test file.
vi.mock("@/components/cardgroups/cardgroup-settings-card", () => ({
  CardgroupSettingsCard: ({ cardgroup }: { cardgroup: { id: string; name: string } }) => (
    <div data-testid="settings-stub">{cardgroup.id}</div>
  ),
}));
vi.mock("@/components/cardgroups/cardgroup-cards-section", () => ({
  CardgroupCardsSection: ({
    cardgroupId,
    initialTotalCount,
  }: {
    cardgroupId: string;
    initialTotalCount: number;
  }) => (
    <div data-testid="cards-section-stub">
      <span data-testid="cards-section-id">{cardgroupId}</span>
      <span data-testid="cards-section-total">{initialTotalCount}</span>
    </div>
  ),
}));

const CARDGROUP = { id: "cg-1", name: "Spanish Vocab" };
const PAGE_INFO = {
  hasNextPage: false,
  hasPreviousPage: false,
  startCursor: null,
  endCursor: null,
};

describe("<CardgroupManagementClient>", () => {
  it("renders the cardgroup name as the h1 page title", () => {
    render(
      <CardgroupManagementClient
        cardgroup={CARDGROUP}
        initialEdges={[]}
        initialPageInfo={PAGE_INFO}
        initialTotalCount={0}
      />,
    );
    expect(screen.getByRole("heading", { level: 1, name: /spanish vocab/i })).toBeInTheDocument();
  });

  it("renders a Back link pointing to /cardgroups", () => {
    render(
      <CardgroupManagementClient
        cardgroup={CARDGROUP}
        initialEdges={[]}
        initialPageInfo={PAGE_INFO}
        initialTotalCount={0}
      />,
    );
    expect(screen.getByRole("link", { name: /back/i })).toHaveAttribute("href", "/cardgroups");
  });

  it("renders both the SettingsCard and CardsSection child slots", () => {
    render(
      <CardgroupManagementClient
        cardgroup={CARDGROUP}
        initialEdges={[]}
        initialPageInfo={PAGE_INFO}
        initialTotalCount={5}
      />,
    );
    expect(screen.getByTestId("settings-stub")).toHaveTextContent("cg-1");
    expect(screen.getByTestId("cards-section-id")).toHaveTextContent("cg-1");
    expect(screen.getByTestId("cards-section-total")).toHaveTextContent("5");
  });

  it("uses a single <main> landmark", () => {
    render(
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
