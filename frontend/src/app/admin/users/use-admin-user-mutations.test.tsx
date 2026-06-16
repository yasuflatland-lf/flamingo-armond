// @vitest-environment jsdom
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

// A user edge: the backend contract sets edge.cursor === user id for the users
// connection (the delete cache filter relies on this).
function userEdge(id: string) {
  return {
    __typename: "UserEdge" as const,
    cursor: id,
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
  it("removes the edge by cursor, decrements totalCount, and evicts on success", async () => {
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
    expect(conn?.users.edges.map((e) => e.cursor)).toEqual(["u-2"]);
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
    expect(conn?.users.edges.map((e) => e.cursor)).toEqual(["u-1", "u-2"]);
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
