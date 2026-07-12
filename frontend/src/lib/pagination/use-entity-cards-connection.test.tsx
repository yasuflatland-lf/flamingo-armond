// @vitest-environment happy-dom
import { MockedProvider } from "@apollo/client/testing/react";
import { renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it } from "vitest";
import {
  CardsByCardgroupConnectionDocument,
  type CardsByCardgroupConnectionQuery,
  type CardsByCardgroupConnectionQueryVariables,
} from "@/generated/graphql";
import {
  defineEntityCardsConnectionConfig,
  useEntityCardsConnection,
} from "./use-entity-cards-connection";

const CARDGROUP_ID = "cg-1";

function cardsDefaultVars(cardgroupId: string): CardsByCardgroupConnectionQueryVariables {
  return { cardgroupId, first: 20, search: null };
}

const CONFIG = defineEntityCardsConnectionConfig({
  document: CardsByCardgroupConnectionDocument,
  connectionField: "cardsByCardgroupConnection",
  defaultVars: cardsDefaultVars,
  logScope: "[test]",
});

type CardEdge = CardsByCardgroupConnectionQuery["cardsByCardgroupConnection"]["edges"][number];

const edge = (id: string): CardEdge => ({
  __typename: "CardEdge",
  cursor: id,
  node: {
    __typename: "Card",
    id,
    front: `front-${id}`,
    back: `back-${id}`,
    userCardState: {
      __typename: "UserCardState",
      due: "2026-01-01T00:00:00Z",
      state: 0,
    },
    cardgroupId: CARDGROUP_ID,
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
            query: CardsByCardgroupConnectionDocument,
            variables: cardsDefaultVars(CARDGROUP_ID),
          },
          result: {
            data: {
              cardsByCardgroupConnection: {
                __typename: "CardConnection",
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

describe("useEntityCardsConnection", () => {
  it("renders prop-seeded edges before the query resolves and exposes queryVariables", () => {
    const { result } = renderHook(
      () =>
        useEntityCardsConnection(CONFIG, {
          ownerId: CARDGROUP_ID,
          searchQuery: null,
          initialEdges: [edge("c-1")],
          initialPageInfo: seedPageInfo,
          initialTotalCount: 1,
          fetchMoreErrorMessage: "fetch-more-failed",
        }),
      { wrapper },
    );
    expect(result.current.edges).toHaveLength(1);
    // searchQuery null → variables are the defaultVars object verbatim (cache-key parity).
    expect(result.current.queryVariables).toEqual(cardsDefaultVars(CARDGROUP_ID));
  });

  it("reads edges out of the config's connection field once the query resolves", async () => {
    const { result } = renderHook(
      () =>
        useEntityCardsConnection(CONFIG, {
          ownerId: CARDGROUP_ID,
          searchQuery: null,
          initialEdges: [],
          initialPageInfo: seedPageInfo,
          initialTotalCount: 0,
          fetchMoreErrorMessage: "fetch-more-failed",
        }),
      { wrapper },
    );
    await waitFor(() => {
      expect(result.current.edges).toHaveLength(1);
      expect(result.current.totalCount).toBe(1);
    });
    expect(result.current.edges[0]?.node.id).toBe("c-1");
  });

  it("keeps queryVariables.search in sync with a non-null search query", async () => {
    const { result } = renderHook(
      () =>
        useEntityCardsConnection(CONFIG, {
          ownerId: CARDGROUP_ID,
          searchQuery: "apple",
          initialEdges: [],
          initialPageInfo: seedPageInfo,
          initialTotalCount: 0,
          fetchMoreErrorMessage: "fetch-more-failed",
        }),
      { wrapper },
    );
    await waitFor(() => {
      expect(result.current.queryVariables.search).toBe("apple");
      expect(result.current.queryVariables.cardgroupId).toBe(CARDGROUP_ID);
    });
  });
});
