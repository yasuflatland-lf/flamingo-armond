// @vitest-environment jsdom
import { MockedProvider } from "@apollo/client/testing/react";
import { screen, waitFor } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { AdminMasterCardsConnectionDocument } from "@/generated/graphql";
import { UndoDeleteProvider } from "@/lib/undo-delete";
import { renderWithIntl } from "@/test/render-with-intl";
import { masterCardsDefaultVars } from "./cards/queries";
import { MasterCardsSection } from "./master-cards-section";

vi.mock("@/components/cardgroups/swipeable-row", async () => {
  const { forwardRef } = await import("react");
  return {
    SwipeableRow: forwardRef(function Mock(
      { children }: { children: React.ReactNode },
      _ref: React.Ref<{ close(): void }>,
    ) {
      return <div>{children}</div>;
    }),
  };
});

const MASTER_ID = "m-1";
const pageInfo = {
  __typename: "PageInfo" as const,
  hasNextPage: false,
  hasPreviousPage: false,
  startCursor: "c-1",
  endCursor: "c-1",
};
const node = {
  __typename: "MasterCard" as const,
  id: "c-1",
  masterCardgroupId: MASTER_ID,
  front: "apple",
  back: "fruit",
  position: 0,
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
};

it("renders MasterCardsClient with the seed and exposes the live count to renderPageHeader", async () => {
  renderWithIntl(
    <MockedProvider
      mocks={[
        {
          request: {
            query: AdminMasterCardsConnectionDocument,
            variables: masterCardsDefaultVars(MASTER_ID),
          },
          result: {
            data: {
              adminMasterCardsConnection: {
                __typename: "MasterCardConnection",
                edges: [{ __typename: "MasterCardEdge", cursor: "c-1", node }],
                pageInfo,
                totalCount: 1,
              },
            },
          },
        },
      ]}
    >
      <UndoDeleteProvider>
        <MasterCardsSection
          masterId={MASTER_ID}
          deckName="Deck One"
          initialEdges={[{ __typename: "MasterCardEdge", cursor: "c-1", node }]}
          initialPageInfo={pageInfo}
          initialTotalCount={1}
          renderPageHeader={({ totalCount }) => <span data-testid="hdr-count">{totalCount}</span>}
        />
      </UndoDeleteProvider>
    </MockedProvider>,
  );
  expect(screen.getByText("apple")).toBeInTheDocument();
  await waitFor(() => expect(screen.getByTestId("hdr-count")).toHaveTextContent("1"));
});
