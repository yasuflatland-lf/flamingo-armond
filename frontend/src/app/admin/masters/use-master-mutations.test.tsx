// @vitest-environment jsdom
import { InMemoryCache } from "@apollo/client";
import type { MockedResponse } from "@apollo/client/testing";
import { MockedProvider } from "@apollo/client/testing/react";
import { act, renderHook } from "@testing-library/react";
import { GraphQLError } from "graphql";
import { createElement, type ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type {
  AdminMastersQuery as AdminMastersQueryResult,
  MasterCardgroupStatus,
} from "@/generated/graphql";
import type { MasterFormValues } from "./admin-master-form";
import {
  ADMIN_MASTERS_BASE_VARS,
  AdminCreateMasterMutation,
  AdminDeleteMasterMutation,
  AdminMastersQuery,
  AdminPublishMasterMutation,
  AdminUnpublishMasterMutation,
  AdminUpdateMasterMutation,
} from "./queries";
import { useMasterMutations } from "./use-master-mutations";

const VALUES: MasterFormValues = {
  name: "Deck A",
  description: null,
  language: null,
  level: null,
  category: null,
  coverImageUrl: null,
  source: null,
  isDefaultStarter: false,
  sortOrder: null,
};

// A fully-populated MasterCardgroup node matching the create/update payload selection set.
function masterNode(
  id: string,
  overrides: { status?: MasterCardgroupStatus; [key: string]: unknown } = {},
) {
  return {
    __typename: "MasterCardgroup" as const,
    id,
    name: "Deck A",
    description: null,
    language: null,
    level: null,
    category: null,
    coverImageUrl: null,
    source: null,
    version: 1,
    status: "DRAFT" as MasterCardgroupStatus,
    isDefaultStarter: false,
    sortOrder: 0,
    cardCount: 0,
    ...overrides,
  };
}

const NULL_SEARCH_VARS = { ...ADMIN_MASTERS_BASE_VARS, search: null };

function render(mocks: MockedResponse[], cache = new InMemoryCache()) {
  return renderHook(() => useMasterMutations(), {
    wrapper: ({ children }: { children: ReactNode }) =>
      createElement(MockedProvider, { mocks, cache }, children),
  });
}

function readConnection(cache: InMemoryCache): AdminMastersQueryResult | null {
  return cache.readQuery<AdminMastersQueryResult>({
    query: AdminMastersQuery,
    variables: NULL_SEARCH_VARS,
  });
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useMasterMutations.createMaster", () => {
  it("returns success and writes a cold-cache connection with the new edge", async () => {
    const cache = new InMemoryCache();
    const mocks: MockedResponse[] = [
      {
        request: { query: AdminCreateMasterMutation, variables: { input: VALUES } },
        result: {
          data: {
            adminCreateMasterCardgroup: {
              __typename: "CreateMasterCardgroupSuccess",
              master: masterNode("m-1"),
            },
          },
        },
      },
    ];
    const { result } = render(mocks, cache);

    let outcome: unknown;
    await act(async () => {
      outcome = await result.current.createMaster(VALUES);
    });

    expect(outcome).toEqual({ status: "success" });
    const conn = readConnection(cache);
    expect(conn?.adminMasters.totalCount).toBe(1);
    expect(conn?.adminMasters.edges).toHaveLength(1);
    expect(conn?.adminMasters.edges.at(0)?.node.id).toBe("m-1");
  });

  it("prepends onto a warm-cache connection and bumps totalCount", async () => {
    const cache = new InMemoryCache();
    cache.writeQuery<AdminMastersQueryResult>({
      query: AdminMastersQuery,
      variables: NULL_SEARCH_VARS,
      data: {
        adminMasters: {
          __typename: "MasterCatalogConnection",
          edges: [{ __typename: "MasterCatalogEdge", cursor: "m-0", node: masterNode("m-0") }],
          pageInfo: {
            __typename: "PageInfo",
            hasNextPage: false,
            hasPreviousPage: false,
            startCursor: "m-0",
            endCursor: "m-0",
          },
          totalCount: 1,
        },
      },
    });
    const mocks: MockedResponse[] = [
      {
        request: { query: AdminCreateMasterMutation, variables: { input: VALUES } },
        result: {
          data: {
            adminCreateMasterCardgroup: {
              __typename: "CreateMasterCardgroupSuccess",
              master: masterNode("m-1"),
            },
          },
        },
      },
    ];
    const { result } = render(mocks, cache);

    await act(async () => {
      await result.current.createMaster(VALUES);
    });

    const conn = readConnection(cache);
    expect(conn?.adminMasters.totalCount).toBe(2);
    expect(conn?.adminMasters.edges.map((e) => e.node.id)).toEqual(["m-1", "m-0"]);
  });

  it("returns validation on InputValidationError", async () => {
    const mocks: MockedResponse[] = [
      {
        request: { query: AdminCreateMasterMutation, variables: { input: VALUES } },
        result: {
          data: {
            adminCreateMasterCardgroup: {
              __typename: "InputValidationError",
              field: "name",
              message: "name is required",
            },
          },
        },
      },
    ];
    const cache = new InMemoryCache();
    const { result } = render(mocks, cache);

    let outcome: unknown;
    await act(async () => {
      outcome = await result.current.createMaster(VALUES);
    });

    expect(outcome).toEqual({ status: "validation", field: "name", message: "name is required" });
    // The update callback must not touch the cache on a validation payload.
    expect(readConnection(cache)).toBeNull();
  });

  it("returns auth(forbidden) when the mutation rejects with FORBIDDEN", async () => {
    const mocks: MockedResponse[] = [
      {
        request: { query: AdminCreateMasterMutation, variables: { input: VALUES } },
        result: { errors: [new GraphQLError("Forbidden", { extensions: { code: "FORBIDDEN" } })] },
      },
    ];
    const { result } = render(mocks);

    let outcome: unknown;
    await act(async () => {
      outcome = await result.current.createMaster(VALUES);
    });

    expect(outcome).toEqual({ status: "auth", kind: "forbidden" });
  });

  it("returns rejected on a transport error", async () => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
    const mocks: MockedResponse[] = [
      {
        request: { query: AdminCreateMasterMutation, variables: { input: VALUES } },
        error: new Error("network down"),
      },
    ];
    const { result } = render(mocks);

    let outcome: unknown;
    await act(async () => {
      outcome = await result.current.createMaster(VALUES);
    });

    expect(outcome).toEqual({ status: "rejected" });
  });

  it("returns unexpected on an unknown payload variant", async () => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
    const mocks: MockedResponse[] = [
      {
        request: { query: AdminCreateMasterMutation, variables: { input: VALUES } },
        result: { data: { adminCreateMasterCardgroup: { __typename: "SomeFutureVariant" } } },
      },
    ];
    const cache = new InMemoryCache();
    const { result } = render(mocks, cache);

    let outcome: unknown;
    await act(async () => {
      outcome = await result.current.createMaster(VALUES);
    });

    expect(outcome).toEqual({ status: "unexpected" });
    // The update callback must not touch the cache on an unknown payload variant.
    expect(readConnection(cache)).toBeNull();
  });
});

describe("useMasterMutations.updateMaster", () => {
  it("returns success on UpdateMasterCardgroupSuccess", async () => {
    const mocks: MockedResponse[] = [
      {
        request: { query: AdminUpdateMasterMutation, variables: { id: "m-1", input: VALUES } },
        result: {
          data: {
            adminUpdateMasterCardgroup: {
              __typename: "UpdateMasterCardgroupSuccess",
              master: masterNode("m-1", { name: "Deck A2" }),
            },
          },
        },
      },
    ];
    const { result } = render(mocks);

    let outcome: unknown;
    await act(async () => {
      outcome = await result.current.updateMaster("m-1", VALUES);
    });

    expect(outcome).toEqual({ status: "success" });
  });

  it("returns validation on InputValidationError", async () => {
    const mocks: MockedResponse[] = [
      {
        request: { query: AdminUpdateMasterMutation, variables: { id: "m-1", input: VALUES } },
        result: {
          data: {
            adminUpdateMasterCardgroup: {
              __typename: "InputValidationError",
              field: "name",
              message: "name too long",
            },
          },
        },
      },
    ];
    const { result } = render(mocks);

    let outcome: unknown;
    await act(async () => {
      outcome = await result.current.updateMaster("m-1", VALUES);
    });

    expect(outcome).toEqual({ status: "validation", field: "name", message: "name too long" });
  });
});

describe("useMasterMutations.deleteMaster", () => {
  it("removes the edge by node.id, decrements totalCount, evicts, returns success", async () => {
    const cache = new InMemoryCache();
    cache.writeQuery<AdminMastersQueryResult>({
      query: AdminMastersQuery,
      variables: NULL_SEARCH_VARS,
      data: {
        adminMasters: {
          __typename: "MasterCatalogConnection",
          edges: [
            { __typename: "MasterCatalogEdge", cursor: "v1:enc-m-1", node: masterNode("m-1") },
            { __typename: "MasterCatalogEdge", cursor: "v1:enc-m-2", node: masterNode("m-2") },
          ],
          pageInfo: {
            __typename: "PageInfo",
            hasNextPage: false,
            hasPreviousPage: false,
            startCursor: "v1:enc-m-1",
            endCursor: "v1:enc-m-2",
          },
          totalCount: 2,
        },
      },
    });
    const mocks: MockedResponse[] = [
      {
        request: { query: AdminDeleteMasterMutation, variables: { id: "m-1" } },
        result: { data: { adminDeleteMasterCardgroup: true } },
      },
    ];
    const { result } = render(mocks, cache);

    let outcome: unknown;
    await act(async () => {
      outcome = await result.current.deleteMaster("m-1");
    });

    expect(outcome).toEqual({ status: "success" });
    const conn = readConnection(cache);
    expect(conn?.adminMasters.totalCount).toBe(1);
    expect(conn?.adminMasters.edges.map((e) => e.node.id)).toEqual(["m-2"]);
    // The deleted entity is evicted + gc'd from the normalized cache.
    expect(cache.extract()["MasterCardgroup:m-1"]).toBeUndefined();
  });

  it("returns rejected when the mutation returns false", async () => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
    const mocks: MockedResponse[] = [
      {
        request: { query: AdminDeleteMasterMutation, variables: { id: "m-1" } },
        result: { data: { adminDeleteMasterCardgroup: false } },
      },
    ];
    const { result } = render(mocks);

    let outcome: unknown;
    await act(async () => {
      outcome = await result.current.deleteMaster("m-1");
    });

    expect(outcome).toEqual({ status: "rejected" });
  });

  it("returns auth(unauthenticated) when the mutation rejects with UNAUTHENTICATED", async () => {
    const mocks: MockedResponse[] = [
      {
        request: { query: AdminDeleteMasterMutation, variables: { id: "m-1" } },
        result: {
          errors: [
            new GraphQLError("Unauthenticated", { extensions: { code: "UNAUTHENTICATED" } }),
          ],
        },
      },
    ];
    const { result } = render(mocks);

    let outcome: unknown;
    await act(async () => {
      outcome = await result.current.deleteMaster("m-1");
    });

    expect(outcome).toEqual({ status: "auth", kind: "unauthenticated" });
  });
});

describe("useMasterMutations.publishMaster / unpublishMaster", () => {
  it("publishMaster returns success on PublishMasterCardgroupSuccess", async () => {
    const mocks: MockedResponse[] = [
      {
        request: { query: AdminPublishMasterMutation, variables: { id: "m-1" } },
        result: {
          data: {
            adminPublishMasterCardgroup: {
              __typename: "PublishMasterCardgroupSuccess",
              master: masterNode("m-1", { status: "PUBLISHED" }),
            },
          },
        },
      },
    ];
    const { result } = render(mocks);

    let outcome: unknown;
    await act(async () => {
      outcome = await result.current.publishMaster("m-1");
    });

    expect(outcome).toEqual({ status: "success" });
  });

  it("publishMaster returns empty on MasterCardgroupEmptyError", async () => {
    const mocks: MockedResponse[] = [
      {
        request: { query: AdminPublishMasterMutation, variables: { id: "m-1" } },
        result: {
          data: {
            adminPublishMasterCardgroup: {
              __typename: "MasterCardgroupEmptyError",
              message: "cannot publish empty deck",
            },
          },
        },
      },
    ];
    const { result } = render(mocks);

    let outcome: unknown;
    await act(async () => {
      outcome = await result.current.publishMaster("m-1");
    });

    expect(outcome).toEqual({ status: "empty" });
  });

  it("unpublishMaster returns success on a truthy payload", async () => {
    const mocks: MockedResponse[] = [
      {
        request: { query: AdminUnpublishMasterMutation, variables: { id: "m-1" } },
        result: { data: { adminUnpublishMasterCardgroup: masterNode("m-1", { status: "DRAFT" }) } },
      },
    ];
    const { result } = render(mocks);

    let outcome: unknown;
    await act(async () => {
      outcome = await result.current.unpublishMaster("m-1");
    });

    expect(outcome).toEqual({ status: "success" });
  });
});
