// @vitest-environment happy-dom
import { InMemoryCache } from "@apollo/client";
import type { MockedResponse } from "@apollo/client/testing";
import { MockedProvider } from "@apollo/client/testing/react";
import { act, renderHook } from "@testing-library/react";
import { GraphQLError } from "graphql";
import { createElement, type ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { AdminUsersQuery as AdminUsersQueryResult } from "@/generated/graphql";
import { ADMIN_USERS_PAGE_SIZE, AdminDeleteUserMutation, AdminUsersQuery } from "./queries";
import { useAdminUserMutations } from "./use-admin-user-mutations";

const USERS_VARS = { first: ADMIN_USERS_PAGE_SIZE, search: null };

// A user edge: the backend emits an opaque "v1:..." cursor (cursor.Encode(id)),
// never the raw user id. The fixture mirrors the encoder so a reverted
// cursor-based delete filter would NOT match the raw id and this test would
// catch it (.claude/rules/pagination.md "Resolve an edge by node.id").
function userEdge(id: string) {
  return {
    __typename: "UserEdge" as const,
    cursor: `v1:${btoa(id)}`,
    node: {
      __typename: "User" as const,
      id,
      displayName: `User ${id}`,
      bio: null,
      avatarUrl: null,
      lastSignInAt: null,
      roles: [],
    },
  };
}

function seedUsers(cache: InMemoryCache, ids: string[]) {
  cache.writeQuery<AdminUsersQueryResult>({
    query: AdminUsersQuery,
    variables: USERS_VARS,
    data: {
      users: {
        __typename: "UserConnection",
        edges: ids.map(userEdge),
        pageInfo: {
          __typename: "PageInfo",
          hasNextPage: false,
          hasPreviousPage: false,
          startCursor: ids[0] ?? null,
          endCursor: ids[ids.length - 1] ?? null,
        },
        totalCount: ids.length,
      },
    },
  });
}

function readUsers(cache: InMemoryCache): AdminUsersQueryResult | null {
  return cache.readQuery<AdminUsersQueryResult>({ query: AdminUsersQuery, variables: USERS_VARS });
}

function render(mocks: MockedResponse[], cache: InMemoryCache) {
  return renderHook(() => useAdminUserMutations(), {
    wrapper: ({ children }: { children: ReactNode }) =>
      createElement(MockedProvider, { mocks, cache }, children),
  });
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useAdminUserMutations.deleteUser", () => {
  it("removes the edge by node id, decrements totalCount, and evicts on success", async () => {
    const cache = new InMemoryCache();
    seedUsers(cache, ["u-1", "u-2"]);
    const mocks: MockedResponse[] = [
      {
        request: { query: AdminDeleteUserMutation, variables: { id: "u-1" } },
        result: { data: { adminDeleteUser: true } },
      },
    ];
    const { result } = render(mocks, cache);

    await act(async () => {
      await result.current.deleteUser("u-1");
    });

    const conn = readUsers(cache);
    expect(conn?.users.totalCount).toBe(1);
    // The surviving edge carries the encoded cursor for u-2 (node.id is masked
    // behind a fragment in the codegen type; cursor is the unmasked top-level
    // field). A reverted cursor-based filter would not have removed u-1, so this
    // also guards the node-id-not-cursor delete behavior.
    expect(conn?.users.edges.map((e) => e.cursor)).toEqual([`v1:${btoa("u-2")}`]);
    // The deleted entity is evicted + gc'd from the normalized cache.
    expect(cache.extract()["User:u-1"]).toBeUndefined();
  });

  it("re-throws and leaves the cache untouched on FORBIDDEN", async () => {
    const cache = new InMemoryCache();
    seedUsers(cache, ["u-1", "u-2"]);
    const mocks: MockedResponse[] = [
      {
        request: { query: AdminDeleteUserMutation, variables: { id: "u-1" } },
        result: { errors: [new GraphQLError("Forbidden", { extensions: { code: "FORBIDDEN" } })] },
      },
    ];
    const { result } = render(mocks, cache);

    await expect(
      act(async () => {
        await result.current.deleteUser("u-1");
      }),
    ).rejects.toThrow();

    const conn = readUsers(cache);
    expect(conn?.users.totalCount).toBe(2);
    expect(conn?.users.edges.map((e) => e.cursor)).toEqual([
      `v1:${btoa("u-1")}`,
      `v1:${btoa("u-2")}`,
    ]);
  });

  it("throws when the mutation returns false", async () => {
    const cache = new InMemoryCache();
    seedUsers(cache, ["u-1"]);
    const mocks: MockedResponse[] = [
      {
        request: { query: AdminDeleteUserMutation, variables: { id: "u-1" } },
        result: { data: { adminDeleteUser: false } },
      },
    ];
    const { result } = render(mocks, cache);

    await expect(
      act(async () => {
        await result.current.deleteUser("u-1");
      }),
    ).rejects.toThrow("adminDeleteUser returned false");
  });
});
