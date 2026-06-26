// @vitest-environment jsdom
import type { MockedResponse } from "@apollo/client/testing";
import { MockedProvider } from "@apollo/client/testing/react";
import { act, renderHook } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MergeMasterCardgroupPreviewQuery } from "@/app/catalog/queries";
import {
  type MergeFromCatalogPreviewOutcome,
  useMergeFromCatalogPreview,
} from "./use-merge-from-catalog-preview";

const TARGET = "cg-1";
const MASTER = "m-1";

function previewMock(payload: Record<string, unknown>): MockedResponse {
  return {
    request: {
      query: MergeMasterCardgroupPreviewQuery,
      variables: { input: { masterCardgroupId: MASTER, cardgroupId: TARGET } },
    },
    result: { data: { mergeMasterCardgroupPreview: payload } },
  };
}

function render(mocks: MockedResponse[]) {
  return renderHook(() => useMergeFromCatalogPreview(TARGET), {
    wrapper: ({ children }: { children: ReactNode }) =>
      createElement(MockedProvider, { mocks }, children),
  });
}

async function runPreview(result: ReturnType<typeof render>["result"]) {
  let outcome: MergeFromCatalogPreviewOutcome | undefined;
  await act(async () => {
    outcome = await result.current.previewMerge(MASTER);
  });
  return outcome as MergeFromCatalogPreviewOutcome;
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useMergeFromCatalogPreview", () => {
  it("returns projected counts on success", async () => {
    const { result } = render([
      previewMock({ __typename: "MergeMasterCardgroupPreview", addedCount: 5, updatedCount: 3 }),
    ]);
    const outcome = await runPreview(result);
    expect(outcome).toEqual({ status: "success", addedCount: 5, updatedCount: 3 });
  });

  it("maps MasterNotFoundError to not_found", async () => {
    const { result } = render([
      previewMock({ __typename: "MasterNotFoundError", message: "gone" }),
    ]);
    const outcome = await runPreview(result);
    expect(outcome).toEqual({ status: "not_found" });
  });

  it("maps a transport error to rejected", async () => {
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    const { result } = render([
      {
        request: {
          query: MergeMasterCardgroupPreviewQuery,
          variables: { input: { masterCardgroupId: MASTER, cardgroupId: TARGET } },
        },
        error: new Error("network"),
      },
    ]);
    const outcome = await runPreview(result);
    expect(outcome.status).toBe("rejected");
    expect(warnSpy).toHaveBeenCalled();
  });
});
