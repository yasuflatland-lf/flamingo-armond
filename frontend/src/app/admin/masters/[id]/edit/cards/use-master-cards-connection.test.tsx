// @vitest-environment jsdom
import { MockedProvider } from "@apollo/client/testing/react";
import { renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it } from "vitest";
import { AdminMasterCardsConnectionDocument } from "@/generated/graphql";
import { masterCardsDefaultVars } from "./queries";
import { useMasterCardsConnection } from "./use-master-cards-connection";

const MASTER_ID = "m-1";
const edge = (id: string) => ({
  __typename: "MasterCardEdge" as const,
  cursor: id,
  node: {
    __typename: "MasterCard" as const,
    id,
    masterCardgroupId: MASTER_ID,
    front: `front-${id}`,
    back: `back-${id}`,
    position: 0,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  },
});

const seedPageInfo = {
  __typename: "PageInfo" as const,
  hasNextPage: false,
  hasPreviousPage: false,
  startCursor: "c-1",
  endCursor: "c-1",
};

function wrapper({ children }: { children: ReactNode }) {
  return (
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
                edges: [edge("c-1")],
                pageInfo: seedPageInfo,
                totalCount: 1,
              },
            },
          },
        },
      ]}
    >
      {children}
    </MockedProvider>
  );
}

describe("useMasterCardsConnection", () => {
  it("renders the prop-seeded edges before the query resolves and exposes queryVariables", () => {
    const { result } = renderHook(
      () =>
        useMasterCardsConnection({
          masterId: MASTER_ID,
          searchQuery: null,
          initialEdges: [edge("c-1")],
          initialPageInfo: seedPageInfo,
          initialTotalCount: 1,
        }),
      { wrapper },
    );
    expect(result.current.edges).toHaveLength(1);
    expect(result.current.queryVariables).toEqual(masterCardsDefaultVars(MASTER_ID));
  });

  it("keeps queryVariables.search in sync with a non-null search query", async () => {
    const { result } = renderHook(
      () =>
        useMasterCardsConnection({
          masterId: MASTER_ID,
          searchQuery: "apple",
          initialEdges: [],
          initialPageInfo: seedPageInfo,
          initialTotalCount: 0,
        }),
      { wrapper },
    );
    await waitFor(() => {
      expect(result.current.queryVariables.search).toBe("apple");
      expect(result.current.queryVariables.masterCardgroupId).toBe(MASTER_ID);
    });
  });
});
