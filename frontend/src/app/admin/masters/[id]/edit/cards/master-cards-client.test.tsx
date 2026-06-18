// @vitest-environment jsdom
import { MockedProvider } from "@apollo/client/testing/react";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { AdminMasterCardsConnectionDocument } from "@/generated/graphql";
import { UndoDeleteProvider } from "@/lib/undo-delete";
import { renderWithIntl } from "@/test/render-with-intl";
import { MasterCardsClient } from "./master-cards-client";
import { masterCardsDefaultVars } from "./queries";

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
const edge = (id: string, front: string) => ({
  __typename: "MasterCardEdge" as const,
  cursor: id,
  node: {
    __typename: "MasterCard" as const,
    id,
    masterCardgroupId: MASTER_ID,
    front,
    back: `${front}-back`,
    position: 0,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  },
});

const seed = {
  request: {
    query: AdminMasterCardsConnectionDocument,
    variables: masterCardsDefaultVars(MASTER_ID),
  },
  result: {
    data: {
      adminMasterCardsConnection: {
        __typename: "MasterCardConnection" as const,
        edges: [edge("c-1", "apple")],
        pageInfo: {
          __typename: "PageInfo" as const,
          hasNextPage: false,
          hasPreviousPage: false,
          startCursor: "c-1",
          endCursor: "c-1",
        },
        totalCount: 1,
      },
    },
  },
};

function render(onTotalCountChange = vi.fn()) {
  renderWithIntl(
    <MockedProvider mocks={[seed]}>
      <UndoDeleteProvider>
        <MasterCardsClient
          masterId={MASTER_ID}
          deckName="Deck One"
          initialEdges={[edge("c-1", "apple")]}
          initialPageInfo={seed.result.data.adminMasterCardsConnection.pageInfo}
          initialTotalCount={1}
          onTotalCountChange={onTotalCountChange}
        />
      </UndoDeleteProvider>
    </MockedProvider>,
  );
  return onTotalCountChange;
}

describe("<MasterCardsClient>", () => {
  it("renders the seeded card and reports the initial total count upward", async () => {
    const onTotalCountChange = render();
    expect(screen.getByText("apple")).toBeInTheDocument();
    await waitFor(() => expect(onTotalCountChange).toHaveBeenCalledWith(1));
  });

  it("opens the add-card sheet from the toolbar", async () => {
    render();
    await userEvent.click(screen.getByRole("button", { name: /add card/i }));
    expect(screen.getByLabelText(/front/i)).toBeInTheDocument();
  });

  it("opens the batch-import sheet from the mobile toolbar button", async () => {
    render();
    await userEvent.click(screen.getByTestId("master-batch-import"));
    // The shared batch-import wizard renders its paste textarea inside the sheet.
    expect(await screen.findByTestId("batch-import-payload")).toBeInTheDocument();
  });
});
