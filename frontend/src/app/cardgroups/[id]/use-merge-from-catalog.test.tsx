// @vitest-environment jsdom
import { ApolloClient } from "@apollo/client";
import type { MockedResponse } from "@apollo/client/testing";
import { MockedProvider } from "@apollo/client/testing/react";
import { act, renderHook } from "@testing-library/react";
import { GraphQLError } from "graphql";
import { createElement, type ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MergeMasterCardgroupMutation } from "@/app/catalog/queries";
import { CardsByCardgroupConnectionDocument } from "@/generated/graphql";
import * as cardsQueries from "./cards/queries";
import { type MergeFromCatalogOutcome, useMergeFromCatalog } from "./use-merge-from-catalog";

const TARGET_CARDGROUP_ID = "cg-target";
const MASTER_CARDGROUP_ID = "cg-master";

const REQUEST = {
  query: MergeMasterCardgroupMutation,
  variables: {
    input: {
      masterCardgroupId: MASTER_CARDGROUP_ID,
      cardgroupId: TARGET_CARDGROUP_ID,
    },
  },
};

function renderMergeFromCatalog(mocks: MockedResponse[]) {
  return renderHook(() => useMergeFromCatalog(TARGET_CARDGROUP_ID), {
    wrapper: ({ children }: { children: ReactNode }) =>
      createElement(MockedProvider, { mocks }, children),
  });
}

async function runMerge(result: ReturnType<typeof renderMergeFromCatalog>["result"]) {
  let outcome: MergeFromCatalogOutcome | undefined;
  await act(async () => {
    outcome = await result.current.mergeFromCatalog(MASTER_CARDGROUP_ID);
  });
  return outcome as MergeFromCatalogOutcome;
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useMergeFromCatalog", () => {
  it("returns success counts and refetches the destination cards query", async () => {
    const cardsDefaultVarsSpy = vi.spyOn(cardsQueries, "cardsDefaultVars");
    const refetchQueriesSpy = vi
      .spyOn(ApolloClient.prototype, "refetchQueries")
      // biome-ignore lint/suspicious/noExplicitAny: test stub for refetchQueries return
      .mockResolvedValue({} as any);
    const mocks: MockedResponse[] = [
      {
        request: REQUEST,
        result: {
          data: {
            mergeMasterCardgroup: {
              __typename: "MergeMasterCardgroupSuccess",
              cardgroup: {
                __typename: "Cardgroup",
                id: TARGET_CARDGROUP_ID,
                name: "Target deck",
                updatedAt: "2026-06-26T00:00:00Z",
              },
              addedCount: 3,
              updatedCount: 1,
            },
          },
        },
      },
    ];

    const { result } = renderMergeFromCatalog(mocks);
    const outcome = await runMerge(result);

    expect(outcome).toEqual({ status: "success", addedCount: 3, updatedCount: 1 });
    expect(cardsDefaultVarsSpy).toHaveBeenCalledWith(TARGET_CARDGROUP_ID);
    expect(refetchQueriesSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        include: [CardsByCardgroupConnectionDocument],
      }),
    );
  });

  it("returns not_found when the server resolves MasterNotFoundError", async () => {
    const mocks: MockedResponse[] = [
      {
        request: REQUEST,
        result: {
          data: {
            mergeMasterCardgroup: {
              __typename: "MasterNotFoundError",
              message: "gone",
            },
          },
        },
      },
    ];

    const { result } = renderMergeFromCatalog(mocks);
    const outcome = await runMerge(result);

    expect(outcome).toEqual({ status: "not_found" });
  });

  it.each([
    ["unauthenticated", "UNAUTHENTICATED"],
    ["forbidden", "FORBIDDEN"],
  ] as const)(
    "returns auth/%s when the mutation rejects with %s",
    async (kind, code) => {
      const mocks: MockedResponse[] = [
        {
          request: REQUEST,
          result: {
            errors: [new GraphQLError("denied", { extensions: { code } })],
          },
        },
      ];

      const { result } = renderMergeFromCatalog(mocks);
      const outcome = await runMerge(result);

      expect(outcome).toEqual({ status: "auth", kind });
    },
  );

  it("returns rejected and logs a scoped warning for a transport error", async () => {
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    const mocks: MockedResponse[] = [
      {
        request: REQUEST,
        error: new Error("network down"),
      },
    ];

    const { result } = renderMergeFromCatalog(mocks);
    const outcome = await runMerge(result);

    expect(outcome).toEqual({ status: "rejected" });
    expect(warnSpy).toHaveBeenCalledWith("[useMergeFromCatalog] mergeMasterCardgroup rejected", {
      targetCardgroupId: TARGET_CARDGROUP_ID,
      name: "Error",
      codes: [],
    });
    expect(JSON.stringify(warnSpy.mock.calls)).not.toContain("network down");
  });

  it("returns rejected for an unexpected union payload", async () => {
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    const mocks: MockedResponse[] = [
      {
        request: REQUEST,
        result: {
          data: {
            mergeMasterCardgroup: {
              __typename: "SomeFutureVariant",
            },
          },
        },
      },
    ];

    const { result } = renderMergeFromCatalog(mocks);
    const outcome = await runMerge(result);

    expect(outcome).toEqual({ status: "rejected" });
    expect(warnSpy).toHaveBeenCalledWith(
      "[useMergeFromCatalog] unexpected mergeMasterCardgroup payload",
      {
        typename: "SomeFutureVariant",
        targetCardgroupId: TARGET_CARDGROUP_ID,
      },
    );
  });
});
